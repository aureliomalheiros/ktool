package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/aureliomalheiros/ktool/internal/kube"
	"github.com/aureliomalheiros/ktool/internal/logs"
	"github.com/aureliomalheiros/ktool/internal/ns"
	"github.com/spf13/cobra"
)

var (
	logsFollow    bool
	logsAllPods   bool
	logsContainer string
	logsTail      int64
	logsSince     string
	logsNamespace string
)

var logsCmd = &cobra.Command{
	Use:   "logs [pod_name]",
	Short: "View logs from a Kubernetes pod",
	Long: `View logs from a pod in the current namespace.

Without a pod name, opens an interactive fzf menu to select a pod (requires fzf).
With a pod name, streams logs directly.

When several pods match the name prefix, use --all (-A) to stream logs from all matching
pods at once (Stern-style), with each line prefixed by [pod-name]. Without --all, fzf
is used to pick one pod when fzf is installed.`,
	Run: func(cmd *cobra.Command, args []string) {
		kc, err := kube.NewKubeClient()
		if err != nil {
			fmt.Printf("Error initializing kube client: %v\n", err)
			os.Exit(1)
		}

		namespace := logsNamespace
		if namespace == "" {
			namespace = ns.CurrentNamespace(kc)
		}

		if len(args) > 0 {
			podName := args[0]
			matches, err := logs.FindMatchingPods(kc, namespace, podName)
			if err != nil {
				fmt.Printf("Error listing pods: %v\n", err)
				os.Exit(1)
			}
			switch len(matches) {
			case 0:
				// No match found; pass the name as-is so the API returns a clear error.
			case 1:
				podName = matches[0]
			default:
				if logsAllPods {
					ctx := context.Background()
					if logsFollow {
						var stop context.CancelFunc
						ctx, stop = signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
						defer stop()
					}
					if err := logs.StreamLogsMulti(ctx, kc, namespace, matches, logsContainer, logsFollow, logsTail, logsSince, os.Stdout); err != nil {
						fmt.Printf("Error streaming logs: %v\n", err)
						os.Exit(1)
					}
					return
				}
				if logs.IsFzfInstalled() {
					selected, err := logs.SelectPodInteractive(matches)
					if err != nil {
						fmt.Printf("Error in interactive selection: %v\n", err)
						os.Exit(1)
					}
					if selected == "" {
						return
					}
					podName = selected
				} else {
					fmt.Printf("Multiple pods match %q:\n", podName)
					logs.PrintPods(kc, matches, namespace)
					os.Exit(1)
				}
			}
			if err := logs.StreamLogs(kc, namespace, podName, logsContainer, logsFollow, logsTail, logsSince); err != nil {
				fmt.Printf("Error streaming logs: %v\n", err)
				os.Exit(1)
			}
			return
		}

		pods, err := logs.ListPods(kc, namespace)
		if err != nil {
			fmt.Printf("Error listing pods: %v\n", err)
			os.Exit(1)
		}

		if len(pods) == 0 {
			fmt.Printf("No pods found in namespace %q\n", namespace)
			return
		}

		if logs.IsFzfInstalled() {
			pod, err := logs.SelectPodInteractive(pods)
			if err != nil {
				fmt.Printf("Error in interactive selection: %v\n", err)
				os.Exit(1)
			}
			if pod == "" {
				return
			}
			if err := logs.StreamLogs(kc, namespace, pod, logsContainer, logsFollow, logsTail, logsSince); err != nil {
				fmt.Printf("Error streaming logs: %v\n", err)
				os.Exit(1)
			}
		} else {
			logs.PrintPods(kc, pods, namespace)
		}
	},
}

func init() {
	logsCmd.Flags().BoolVarP(&logsFollow, "follow", "f", false, "Follow log output")
	logsCmd.Flags().BoolVarP(&logsAllPods, "all", "A", false, "When multiple pods match, stream logs from all of them (Stern-style)")
	logsCmd.Flags().StringVarP(&logsContainer, "container", "c", "", "Container name (for multi-container pods)")
	logsCmd.Flags().Int64Var(&logsTail, "tail", 50, "Number of lines from the end of the logs (-1 for all)")
	logsCmd.Flags().StringVar(&logsSince, "since", "", "Show logs since duration (e.g. 1h, 30m, 5s)")
	logsCmd.Flags().StringVarP(&logsNamespace, "namespace", "n", "", "Namespace (defaults to current namespace)")
	rootCmd.AddCommand(logsCmd)
}
