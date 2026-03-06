package cmd

import (
	"fmt"
	"os"

	"github.com/aureliomalheiros/ktool/internal/kube"
	"github.com/aureliomalheiros/ktool/internal/ns"
	"github.com/spf13/cobra"
)

var nsCmd = &cobra.Command{
	Use:   "ns [namespace_name]",
	Short: "List and switch Kubernetes namespaces",
	Run: func(cmd *cobra.Command, args []string) {
		kc, err := kube.NewKubeClient()
		if err != nil {
			fmt.Printf("Error initializing kube client: %v\n", err)
			os.Exit(1)
		}

		namespaces, err := ns.ListNamespaces(kc)
		if err != nil {
			fmt.Printf("Error listing namespaces: %v\n", err)
			os.Exit(1)
		}

		if len(args) > 0 {
			target := args[0]
			current := ns.CurrentNamespace(kc)
			if target == current {
				fmt.Printf("Already on namespace \"%s\"\n", target)
				return
			}
			if err := ns.SetNamespace(kc, target); err != nil {
				fmt.Printf("Error switching namespace: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Switched to namespace \"%s\"\n", target)
			return
		}

		if ns.IsFzfInstalled() {
			if err := ns.SelectNamespaceInteractive(kc, namespaces); err != nil {
				fmt.Printf("Error in interactive selection: %v\n", err)
				os.Exit(1)
			}
		} else {
			ns.PrintNamespaces(kc, namespaces)
		}
	},
}

func init() {
	rootCmd.AddCommand(nsCmd)
}
