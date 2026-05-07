package broker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/ibeloyar/gophprofile/internal/model"
	"go.uber.org/zap"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Publisher struct {
	lg *zap.SugaredLogger

	conn    *amqp.Connection
	channel *amqp.Channel

	confirms chan amqp.Confirmation
}

func NewPublisher(lg *zap.SugaredLogger, url string) (*Publisher, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, err
	}
	//defer conn.Close()

	// Создание канала для работы с очередями и сообщениями
	// Канал — это виртуальное соединение внутри TCP-соединения
	ch, err := conn.Channel()
	if err != nil {
		return nil, err
	}
	//defer ch.Close()

	// Включаем режим подтверждений публикаций
	if err := ch.Confirm(false); err != nil {
		log.Fatalf("enable confirms: %v", err)
	}

	return &Publisher{
		lg:       lg,
		conn:     conn,
		channel:  ch,
		confirms: make(chan amqp.Confirmation, 1),
	}, nil
}

func (p *Publisher) Init() error {
	go p.handleConfirms()

	// Создаём exchange
	if err := p.channel.ExchangeDeclare(exchangeName, "direct", true, false, false, false, nil); err != nil {
		return err
	}

	return nil
}

// Health проверяет, что соединение с RabbitMQ активно
func (p *Publisher) Health() error {
	if p.conn == nil || p.conn.IsClosed() {
		return errors.New("rabbitMQ connection is closed")
	}

	// Дополнительно можно проверить канал
	if p.channel == nil || p.channel.IsClosed() {
		return errors.New("rabbitMQ channel is closed")
	}

	return nil
}

func (p *Publisher) Shutdown() error {
	if err := p.channel.Close(); err != nil {
		return err
	}

	if err := p.conn.Close(); err != nil {
		return err
	}

	return nil
}

func (p *Publisher) PublishUpload(ctx context.Context, event *model.AvatarUploadEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	if err := p.channel.PublishWithContext(ctx, exchangeName, uploadKey, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         body,
	}); err != nil {
		return err
	}

	return nil
}

func (p *Publisher) PublishDelete(ctx context.Context, event *model.AvatarDeleteEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	if err := p.channel.PublishWithContext(ctx, exchangeName, deleteKey, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         body,
	}); err != nil {
		return err
	}

	return nil
}

func (p *Publisher) handleConfirms() {
	confirms := p.channel.NotifyPublish(p.confirms)
	for confirm := range confirms {
		if confirm.Ack {
			p.lg.Debugw("message confirmed", "delivery_tag", confirm.DeliveryTag)
		} else {
			p.lg.Errorw("message nacked", "delivery_tag", confirm.DeliveryTag)
		}
	}
}
