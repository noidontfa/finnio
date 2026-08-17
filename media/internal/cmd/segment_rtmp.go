package cmd

import (
	"context"
	"fmt"
	"log"
	"media/internal/abr"
	"media/internal/ffmpeg"
	"media/internal/segment"
	"os"
	"os/signal"
	"shared/platform"
	"strconv"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/spf13/cobra"
)

var (
	rtmpURL       string
	rtmpOutFolder string
	rtmpHlsTime   int
	rtmpWaitSecs  int
	rtmpSourceID  string
)

var segmentRtmpCmd = &cobra.Command{
	Use:   "segment-rtmp",
	Short: "Split an RTMP ingest into source segments (-c copy)",
	Run: func(cmd *cobra.Command, args []string) {
		segmentRtmp()
	},
}

func init() {
	segmentRtmpCmd.Flags().StringVarP(&rtmpURL, "input", "i", "", "RTMP URL example: rtmp://127.0.0.1:1935/live/key")
	segmentRtmpCmd.Flags().StringVarP(&rtmpOutFolder, "output", "o", "", "output folder example: output")
	segmentRtmpCmd.Flags().IntVarP(&rtmpHlsTime, "hls-time", "t", 2, "segment time in seconds")
	segmentRtmpCmd.Flags().IntVar(&rtmpWaitSecs, "wait", 10, "seconds to wait for RTMP video before failing")
	segmentRtmpCmd.Flags().StringVarP(&rtmpSourceID, "source-id", "s", "", "source ID example: source_id")
}

func segmentRtmp() {
	if rtmpURL == "" {
		log.Fatal("input RTMP URL is required")
	}
	if rtmpOutFolder == "" {
		log.Fatal("output folder is required")
	}
	if rtmpHlsTime <= 0 {
		log.Fatal("segment time must be > 0")
	}
	if rtmpWaitSecs <= 0 {
		log.Fatal("wait must be > 0")
	}

	fmt.Println("input:", rtmpURL)
	fmt.Println("output:", rtmpOutFolder)
	fmt.Println("hlsTime:", rtmpHlsTime)
	fmt.Println("wait:", rtmpWaitSecs, "s")
	fmt.Println("--------------------------------")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	runner := ffmpeg.NewRunner()
	segmenter := segment.NewSegmenter(runner)
	cfg := platform.Load()
	nc, err := nats.Connect(cfg.NatsURL)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	js, err := nc.JetStream()
	if err != nil {
		log.Fatalf("Failed to create JetStream context: %v", err)
	}
	pub := abr.NewPublisher(js)
	if err != nil {
		log.Fatalf("Failed to create ABR: %v", err)
	}

	fmt.Println("waiting for RTMP video…")
	if err := segmenter.WaitForVideo(ctx, rtmpURL, 0); err != nil {
		log.Fatal(err)
	}
	fmt.Println("RTMP video ready")

	err = segmenter.SourceContext(ctx,
		segment.SourceOptions{
			InputFile:    rtmpURL,
			OutputFolder: rtmpOutFolder,
			HlsTime:      strconv.Itoa(rtmpHlsTime),
			Live:         true,
		},
		segment.Hooks{
			OnSegmentCreated: func(segmentFile string) error {
				fmt.Println("[Hook] segment created", segmentFile)
				err := pub.Publish(abr.Request{
					SourceID:    rtmpSourceID,
					SourceType:  "video",
					SegmentFile: segmentFile,
					Timestamp:   time.Now(),
				})
				if err != nil {
					return err
				}
				return nil
			},
			OnSegmenterReady: func(outputFolder string) error {
				fmt.Println("[Hook] segmenter ready", outputFolder)
				return nil
			},
			OnSegmenterDone: func(outputFolder string) error {
				fmt.Println("[Hook] segmenter done", outputFolder)
				err := pub.Publish(abr.Request{
					SourceID:    rtmpSourceID,
					SourceType:  "video_done",
					SegmentFile: "",
					Timestamp:   time.Now(),
				})
				if err != nil {
					return err
				}
				return nil
			},
		})
	if err != nil {
		log.Fatal("failed to segment RTMP: ", err)
	}
	fmt.Println("RTMP segmentation finished")
}
