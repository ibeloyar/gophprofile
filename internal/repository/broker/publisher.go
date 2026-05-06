package broker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/ibeloyar/gophprofile/internal/model"
	amqp "github.com/rabbitmq/amqp091-go"
)

func NewPublisher(url string) (*Publisher, error) {
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

	// Канал подтверждений
	//confirms := ch.NotifyPublish(make(chan amqp.Confirmation, 1))

	//// Создаём exchange и очередь
	//if err := ch.ExchangeDeclare("events", "direct", true, false, false, false, nil); err != nil {
	//	log.Fatal(err)
	//}
	//if _, err := ch.QueueDeclare("events.q", true, false, false, false, nil); err != nil {
	//	log.Fatal(err)
	//}
	//if err := ch.QueueBind("events.q", "created", "events", false, nil); err != nil {
	//	log.Fatal(err)
	//}

	// Публикуем сообщение с таймаутом
	//ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	//defer cancel()
	//
	//select {
	//case c := <-confirms:
	//	if c.Ack {
	//		log.Println("broker ACK: сообщение принято")
	//	} else {
	//		log.Println("broker NACK: брокер отверг публикацию, можно ретраить")
	//	}
	//case <-ctx.Done():
	//	log.Println("timeout ожидания confirm — неизвестно, принято ли сообщение")
	//}

	return &Publisher{
		conn:    conn,
		channel: ch,
	}, nil
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

	return p.channel.PublishWithContext(ctx, exchangeName, uploadKey, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         body,
	})
}

func (p *Publisher) PublishDelete(ctx context.Context, event *model.AvatarDeleteEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	return p.channel.PublishWithContext(ctx, exchangeName, deleteKey, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         body,
	})
}
