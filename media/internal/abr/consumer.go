package abr

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/nats-io/nats.go"
)

type ConsumerOptions struct {
	NumConsumers int
	OutputFolder string
}

type Consumer struct {
	js nats.JetStreamContext
}

type ConsumerHook struct {
	OnRequest func(consumer int, request Request, outputFolder string) error
}

func NewConsumer(js nats.JetStreamContext) (*Consumer, error) {
	_, err := js.AddStream(&nats.StreamConfig{
		Name:     ARB_STREAM,
		Subjects: []string{ARB_SUBJECT},
	})
	if err != nil {
		return nil, err
	}

	return &Consumer{js: js}, nil
}

func (c *Consumer) Run(options ConsumerOptions, hook ConsumerHook) error {
	if options.NumConsumers < 1 {
		return fmt.Errorf("numConsumers must be >= 1")
	}

	for i := 0; i < options.NumConsumers; i++ {
		worker := i
		_, err := c.js.QueueSubscribe(
			ARB_SUBJECT_REQUESTS,
			ARB_WORKER,
			func(msg *nats.Msg) {
				d := Request{}
				err := json.Unmarshal(msg.Data, &d)
				if err != nil {
					log.Printf("worker %d: bad request: %v", worker, err)
					msg.Nak()
					return
				}
				log.Printf("worker %d: %+v", worker, d)

				err = hook.OnRequest(worker, d, options.OutputFolder)
				if err != nil {
					log.Printf("worker %d: on request failed: %v", worker, err)
					return
				}
				msg.Ack()
			},
			nats.Durable(ARB_WORKER),
			nats.BindStream(ARB_STREAM),
			nats.ManualAck(),
			nats.AckExplicit(),
		)
		if err != nil {
			return err
		}
	}

	return nil
}
