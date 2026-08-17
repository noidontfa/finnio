package cmd

import (
	"fmt"
	"log"
	"media/internal/abr"
	"media/internal/ffmpeg"
	"path"
	"shared/platform"

	"github.com/nats-io/nats.go"
	"github.com/spf13/cobra"
)

var abrConsumerWorkers int
var abrOutFolder string

var abrConsumerCmd = &cobra.Command{
	Use:   "abr-consumer",
	Short: "ABR consumer",
	Run: func(cmd *cobra.Command, args []string) {
		AbrConsumer()
	},
}

func init() {
	abrConsumerCmd.Flags().IntVarP(&abrConsumerWorkers, "consumers", "c", 5, "number of concurrent ABR consumers")
	abrConsumerCmd.Flags().StringVarP(&abrOutFolder, "output", "o", "", "ABR output folder, e.g. abr-test")
}

func AbrConsumer() {
	cfg := platform.Load()
	nc, err := nats.Connect(cfg.NatsURL)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	js, err := nc.JetStream()
	consumer, err := abr.NewConsumer(js)
	if err != nil {
		log.Fatalf("Failed to create ABR consumer: %v", err)
	}
	hdl := abr.NewHandler(ffmpeg.NewRunner())
	if err != nil {
		log.Fatalf("Failed to create ABR handler: %v", err)
	}
	err = consumer.Run(
		abr.ConsumerOptions{
			NumConsumers: abrConsumerWorkers,
			OutputFolder: abrOutFolder,
		},
		abr.ConsumerHook{
			OnRequest: func(consumer int, request abr.Request, outputFolder string) error {
				fmt.Printf("consumer %d: on request %+v, output folder: %s\n", consumer, request, outputFolder)
				return hdl.Handle(request, path.Join(outputFolder, request.SourceID))
			},
		})
	if err != nil {
		log.Fatalf("Failed to create ABR: %v", err)
	}
	log.Println("ABR consumer started")
	select {}
}
