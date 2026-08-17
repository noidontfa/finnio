package abr

import (
	"encoding/json"
	"fmt"

	"github.com/nats-io/nats.go"
)

type Publisher struct {
	js nats.JetStreamContext
}

func NewPublisher(js nats.JetStreamContext) *Publisher {
	return &Publisher{js: js}
}

func (p *Publisher) Publish(data Request) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	_, err = p.js.Publish(ARB_SUBJECT_REQUESTS, jsonData)
	if err != nil {
		return fmt.Errorf("failed to publish data: %w", err)
	}
	return nil
}
