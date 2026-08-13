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
}
