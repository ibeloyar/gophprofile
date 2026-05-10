package broker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/google/uuid"
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

// NewPublisher establishes RabbitMQ connection and channel with confirms enabled.
func NewPublisher(lg *zap.SugaredLogger, url string) (*Publisher, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		return nil, err
	}

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

// Init declares 'avatars.exchange' (direct, durable) and starts confirm handler.
func (p *Publisher) Init() error {
	go p.handleConfirms()

	if err := p.channel.ExchangeDeclare(exchangeName, "direct", true, false, false, false, nil); err != nil {
		return err
	}

	return nil
}

// Health checks RabbitMQ connection and channel status.
func (p *Publisher) Health() error {
	if p.conn == nil || p.conn.IsClosed() {
		return errors.New("rabbitMQ connection is closed")
	}
	if p.channel == nil || p.channel.IsClosed() {
		return errors.New("rabbitMQ channel is closed")
	}

	return nil
}

// Shutdown closes channel and connection gracefully.
func (p *Publisher) Shutdown() error {
	if err := p.channel.Close(); err != nil {
		return err
	}
	if err := p.conn.Close(); err != nil {
		return err
	}

	return nil
}

// PublishUpload publishes avatar upload event to 'avatars.exchange' with upload routing key.
// Message is persistent and confirmed before return.
func (p *Publisher) PublishUpload(ctx context.Context, event *model.AvatarUploadEvent) error {
	event.MessageID = uuid.NewString()

	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	if err := p.channel.PublishWithContext(ctx, exchangeName, uploadKey, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         body,
		MessageId:    event.MessageID,
	}); err != nil {
		return err
	}

	return nil
}

// PublishDelete publishes avatar delete event to 'avatars.exchange' with delete routing key.
// Message is persistent and confirmed before return.
func (p *Publisher) PublishDelete(ctx context.Context, event *model.AvatarDeleteEvent) error {
	event.MessageID = uuid.NewString()

	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	if err := p.channel.PublishWithContext(ctx, exchangeName, deleteKey, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         body,
		MessageId:    event.MessageID,
	}); err != nil {
		return err
	}

	return nil
}

// handleConfirms processes publisher confirmations asynchronously.
// Logs ACK/NACK for each message with delivery tag.
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
