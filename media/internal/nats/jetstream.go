package nats

import (
	"github.com/nats-io/nats.go"
)

type JetStream struct {
	nc *nats.Conn
	js nats.JetStreamContext
}

func NewJetStream(url string) (*JetStream, error) {
	nc, err := nats.Connect(url)
	if err != nil {
		return nil, err
	}
	js, err := nc.JetStream()
	if err != nil {
		return nil, err
	}
	return &JetStream{nc: nc, js: js}, nil
}

func (j *JetStream) Publish(subject string, data []byte) error {
	_, err := j.js.Publish(subject, data)
	return err
}

func (j *JetStream) Subscribe(subject string, callback func(msg *nats.Msg)) error {
	_, err := j.js.Subscribe(subject, func(msg *nats.Msg) {
		callback(msg)
	})
	return err
}
