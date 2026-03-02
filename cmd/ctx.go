package cmd

import (
	"fmt"
	"os"

	"github.com/aureliomalheiros/ktool/internal/ctx"
	"github.com/aureliomalheiros/ktool/internal/kube"
	"github.com/spf13/cobra"
)

var ctxCmd = &cobra.Command{
	Use:   "ctx [context_name]",
	Short: "List and manage Kubernetes contexts",
	Run: func(cmd *cobra.Command, args []string) {
		kc, err := kube.NewKubeClient()
		if err != nil {
			fmt.Printf("Error initializing kube client: %v\n", err)
			os.Exit(1)
		}

		if len(args) > 0 {
			target := args[0]
			current := ctx.CurrentContext(kc)
			if target == current {
				fmt.Printf("Already on context \"%s\"\n", target)
				return
			}
			if err := ctx.SwitchContext(kc, target); err != nil {
				fmt.Printf("Error switching context: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Switched to context \"%s\"\n", target)
			return
		}

		if ctx.IsFzfInstalled() {
			if err := ctx.SelectContextInteractive(kc); err != nil {
				fmt.Printf("Error in interactive selection: %v\n", err)
				os.Exit(1)
			}
		} else {
			ctx.PrintContexts(kc)
		}
	},
}

func init() {
	rootCmd.AddCommand(ctxCmd)
}
