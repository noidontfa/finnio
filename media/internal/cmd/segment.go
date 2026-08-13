package cmd

import (
	"context"
	"fmt"
	"log"
	"media/internal/ffmpeg"
	"media/internal/segment"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/spf13/cobra"
)

var (
	inputFile    string
	outputFolder string
	hlsTime      int
)

var segmentCms = &cobra.Command{
	Use:   "segment",
	Short: "Split source file into segments",
	Run: func(cmd *cobra.Command, args []string) {
		segmentCmd()
	},
}

func init() {
	segmentCms.Flags().StringVarP(&inputFile, "input", "i", "", "input file example: input.mp4")
	segmentCms.Flags().StringVarP(&outputFolder, "output", "o", "", "output folder example: output")
	segmentCms.Flags().IntVarP(&hlsTime, "hls-time", "t", 2, "segment time in seconds")
}

func segmentCmd() {
	if inputFile == "" {
		log.Fatal("input file is required")
	}
	if outputFolder == "" {
		log.Fatal("output folder is required")
	}
	if hlsTime <= 0 {
		log.Fatal("segment time is required")
	}
	fmt.Println("inputFile:", inputFile)
	fmt.Println("outputFolder:", outputFolder)
	fmt.Println("hlsTime:", hlsTime)
	fmt.Println("--------------------------------")

	runner := ffmpeg.NewRunner()
	segmenter := segment.NewSegmenter(runner)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	err := segmenter.SourceContext(ctx,
		segment.SourceOptions{
			InputFile:    inputFile,
			OutputFolder: outputFolder,
			HlsTime:      strconv.Itoa(hlsTime),
		},
		segment.Hooks{
			OnSegmentCreated: func(segmentFile string) error {
				fmt.Println("segment created", segmentFile)
				// Send segment file to Nats
				return nil
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
