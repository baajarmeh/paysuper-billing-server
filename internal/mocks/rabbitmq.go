package mocks

import (
	"errors"
	"github.com/golang/protobuf/proto"
	"github.com/streadway/amqp"
	rabbitmq "gopkg.in/ProtocolONE/rabbitmq.v1/pkg"
)

const (
	SomeError = "some error"
)

type BrokerMockOk struct {
	IsSent bool
}
type BrokerMockError struct{}

func NewBrokerMockOk() rabbitmq.BrokerInterface {
	return &BrokerMockOk{}
}

func NewBrokerMockError() rabbitmq.BrokerInterface {
	return &BrokerMockError{}
}

func (b *BrokerMockOk) RegisterSubscriber(topic string, fn interface{}) error {
	return nil
}

func (b *BrokerMockOk) Subscribe(exit chan bool) error {
	return nil
}

func (b *BrokerMockOk) Publish(topic string, msg proto.Message, h amqp.Table) error {
	b.IsSent = true
	return nil
}

func (b *BrokerMockOk) SetExchangeName(name string) {}

func (b *BrokerMockOk) SetQueueOptsArgs(args amqp.Table) {}

func (b *BrokerMockError) RegisterSubscriber(topic string, fn interface{}) error {
	return errors.New(SomeError)
}

func (b *BrokerMockError) Subscribe(exit chan bool) error {
	return errors.New(SomeError)
}

func (b *BrokerMockError) Publish(topic string, msg proto.Message, h amqp.Table) (err error) {
	return errors.New(SomeError)
}

func (b *BrokerMockError) SetExchangeName(name string) {}

func (b *BrokerMockError) SetQueueOptsArgs(args amqp.Table) {}
