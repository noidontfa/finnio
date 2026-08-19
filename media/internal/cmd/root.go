package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "media",
	Short: "Media CLI",
}

func Execute() error {
	if err := rootCmd.Execute(); err != nil {
		return err
	}
	return nil
}

func init() {
	rootCmd.AddCommand(helloworldCmd)
	rootCmd.AddCommand(segmentCms)
	rootCmd.AddCommand(segmentRtmpCmd)
	rootCmd.AddCommand(abrCmd)
	rootCmd.AddCommand(abrConsumerCmd)
	rootCmd.AddCommand(segment2Cms)
	rootCmd.AddCommand(segmentRtmp2Cmd)
}
