package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var helloName string

var helloworldCmd = &cobra.Command{
	Use:   "hello",
	Short: "Hello world",
	Run: func(cmd *cobra.Command, args []string) {
		HelloWorld(helloName)
	},
}

func init() {
	helloworldCmd.Flags().StringVarP(&helloName, "name", "n", "World", "name to greet")
}

func HelloWorld(name string) {
	fmt.Printf("Hello, %s!\n", name)
}
