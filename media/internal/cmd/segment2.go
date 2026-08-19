package cmd

import (
	"fmt"
	"log"
	"media/internal/abr"
	"media/internal/ffmpeg"
	segmentv2 "media/internal/segment_v2"
	"shared/platform"
	"strconv"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/spf13/cobra"
)

var (
	inputFile2    string
	outputFolder2 string
	hlsTime2      int
	sourceID2     string
)

var segment2Cms = &cobra.Command{
	Use:   "segment2",
	Short: "Split source file into segments",
	Run: func(cmd *cobra.Command, args []string) {
		segment2Cmd()
	},
}

func init() {
	segment2Cms.Flags().StringVarP(&inputFile2, "input", "i", "", "input file example: input.mp4")
	segment2Cms.Flags().StringVarP(&outputFolder2, "output", "o", "", "output folder example: output")
	segment2Cms.Flags().IntVarP(&hlsTime2, "hls-time", "t", 2, "segment time in seconds")
	segment2Cms.Flags().StringVarP(&sourceID2, "source-id", "s", "", "source ID example: source_id")
}

func segment2Cmd() {
	if inputFile2 == "" {
		log.Fatal("input file is required")
	}
	if outputFolder2 == "" {
		log.Fatal("output folder is required")
	}
	if hlsTime2 <= 0 {
		log.Fatal("segment time is required")
	}
	fmt.Println("inputFile:", inputFile2)
	fmt.Println("outputFolder:", outputFolder2)
	fmt.Println("hlsTime:", hlsTime2)
	fmt.Println("--------------------------------")

	cfg := platform.Load()
	runner := ffmpeg.NewRunner()
	segmenter := segmentv2.NewSegmenter(runner)
	nc, err := nats.Connect(cfg.NatsURL)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	js, err := nc.JetStream()
	if err != nil {
		log.Fatalf("Failed to create JetStream context: %v", err)
	}
	pub := abr.NewPublisher(js)

	err = segmenter.Source(
		segmentv2.Options{
			InputFile:    inputFile2,
			OutputFolder: outputFolder2,
			HlsTime:      strconv.Itoa(hlsTime2),
		},
		segmentv2.Hooks{
			OnSegmentCreated: func(tsFile string) error {
				fmt.Println("segment created", tsFile)
				return pub.Publish(abr.Request{
					SourceID:    sourceID2,
					SourceType:  abr.TS_FILE,
					SegmentFile: tsFile,
					Timestamp:   time.Now(),
				})
			},
			OnMasterUpdated: func(masterFile string) error {
				fmt.Println("master updated", masterFile)
				return pub.Publish(abr.Request{
					SourceID:    sourceID2,
					SourceType:  abr.INDEX_FILE,
					SegmentFile: masterFile,
					Timestamp:   time.Now(),
				})
			},
			OnSegmenterReady: func(outputFolder string) error {
				fmt.Println("segmenter ready", outputFolder, "starting to upload")
				return nil
			},
		})

	if err != nil {
		log.Fatal("failed to segment source file: ", err)
	}
	fmt.Println("source file segmented successfully")
}
