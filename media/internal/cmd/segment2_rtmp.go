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
	rtmpURL2       string
	rtmpOutFolder2 string
	rtmpHlsTime2   int
	rtmpWaitSecs2  int
	rtmpSourceID2  string
)

var segmentRtmp2Cmd = &cobra.Command{
	Use:   "segment2-rtmp",
	Short: "Split an RTMP ingest into source segments (-c copy)",
	Run: func(cmd *cobra.Command, args []string) {
		segmentRtmp2()
	},
}

func init() {
	segmentRtmp2Cmd.Flags().StringVarP(&rtmpURL2, "input", "i", "", "RTMP URL example: rtmp://127.0.0.1:1935/live/key")
	segmentRtmp2Cmd.Flags().StringVarP(&rtmpOutFolder2, "output", "o", "", "output folder example: output")
	segmentRtmp2Cmd.Flags().IntVarP(&rtmpHlsTime2, "hls-time", "t", 2, "segment time in seconds")
	segmentRtmp2Cmd.Flags().IntVar(&rtmpWaitSecs2, "wait", 10, "seconds to wait for RTMP video before failing")
	segmentRtmp2Cmd.Flags().StringVarP(&rtmpSourceID2, "source-id", "s", "", "source ID example: source_id")
}

func segmentRtmp2() {
	if rtmpURL2 == "" {
		log.Fatal("input RTMP URL is required")
	}
	if rtmpOutFolder2 == "" {
		log.Fatal("output folder is required")
	}
	if rtmpHlsTime2 <= 0 {
		log.Fatal("segment time must be > 0")
	}
	if rtmpWaitSecs2 <= 0 {
		log.Fatal("wait must be > 0")
	}

	fmt.Println("input:", rtmpURL2)
	fmt.Println("output:", rtmpOutFolder2)
	fmt.Println("hlsTime:", rtmpHlsTime2)
	fmt.Println("wait:", rtmpWaitSecs2, "s")
	fmt.Println("--------------------------------")

	runner := ffmpeg.NewRunner()
	segmenter := segmentv2.NewSegmenter(runner)
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

	fmt.Println("waiting for RTMP video…")
	if err := segmenter.WaitForVideo(rtmpURL2, time.Duration(rtmpWaitSecs2)*time.Second); err != nil {
		log.Fatal(err)
	}
	fmt.Println("RTMP video ready")

	err = segmenter.Source(
		segmentv2.Options{
			InputFile:    rtmpURL2,
			OutputFolder: rtmpOutFolder2,
			HlsTime:      strconv.Itoa(rtmpHlsTime2),
		},
		segmentv2.Hooks{
			OnSegmentCreated: func(tsFile string) error {
				fmt.Println("segment created", tsFile)
				return pub.Publish(abr.Request{
					SourceID:    rtmpSourceID2,
					SourceType:  abr.TS_FILE,
					SegmentFile: tsFile,
					Timestamp:   time.Now(),
				})
			},
			OnMasterUpdated: func(masterFile string) error {
				fmt.Println("master updated", masterFile)
				return pub.Publish(abr.Request{
					SourceID:    rtmpSourceID2,
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
		log.Fatal("failed to segment RTMP: ", err)
	}
	fmt.Println("RTMP segmentation finished")
}
