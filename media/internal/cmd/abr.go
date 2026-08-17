package cmd

import (
	"log"
	"media/internal/abr"
	"media/internal/ffmpeg"
	"path"
	"shared/platform"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/spf13/cobra"
)

var (
	abrConsumers int
	abrInput     string
	abrOutput    string
)

var abrCmd = &cobra.Command{
	Use:   "abr",
	Short: "ABR (Adaptive Bitrate Streaming) server",
	Run: func(cmd *cobra.Command, args []string) {
		Abr()
	},
}

func init() {
	abrCmd.Flags().IntVarP(&abrConsumers, "consumers", "c", 2, "number of concurrent ABR consumers")
	abrCmd.Flags().StringVarP(&abrInput, "input", "i", "", "source segment, e.g. seg_00003.ts")
	abrCmd.Flags().StringVarP(&abrOutput, "output", "o", "", "ABR output folder, e.g. abr-test")
}

func Abr() {
	if abrInput == "" {
		log.Fatal("input file is required")
	}
	if abrOutput == "" {
		log.Fatal("output folder is required")
	}
	if abrConsumers < 1 {
		log.Fatal("consumers must be >= 1")
	}

	cfg := platform.Load()
	nc, err := nats.Connect(cfg.NatsURL)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	js, err := nc.JetStream()
	if err != nil {
		log.Fatalf("Failed to create JetStream context: %v", err)
	}
	con, err := abr.NewConsumer(js)
	if err != nil {
		log.Fatalf("Failed to create ABR consumer: %v", err)
	}
	hdl := abr.NewHandler(ffmpeg.NewRunner())
	if err != nil {
		log.Fatalf("Failed to create ABR handler: %v", err)
	}

	err = con.Run(
		abr.ConsumerOptions{
			NumConsumers: abrConsumers,
			OutputFolder: abrOutput,
		},
		abr.ConsumerHook{
			OnRequest: func(consumer int, request abr.Request, outputFolder string) error {
				log.Printf("consumer %d: on request %+v, output folder: %s\n", consumer, request, outputFolder)
				return hdl.Handle(request, path.Join(outputFolder, request.SourceID))
			},
		})
	if err != nil {
		log.Fatalf("Failed to run ABR consumer: %v", err)
	}
	log.Println("ABR consumer started")

	pub := abr.NewPublisher(js)
	log.Println("ABR publisher started")
	err = pub.Publish(abr.Request{
		SourceType:  "video",
		SegmentFile: abrInput,
		Timestamp:   time.Now(),
	})
	if err != nil {
		log.Println("Failed to publish ABR request: %v", err)
	}
	log.Println("ABR request published")
	select {
	case <-time.After(10 * time.Second):
		log.Println("ABR server stopped")
		return
	}
}
