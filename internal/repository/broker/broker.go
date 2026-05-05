package broker

import (
	amqp "github.com/rabbitmq/amqp091-go"
)

type Publisher struct {
	conn    *amqp.Connection
	channel *amqp.Channel
}

const (
	exchangeName = "avatars.exchange"
	uploadKey    = "avatar.uploaded"
)
