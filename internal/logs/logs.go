package logs

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/aureliomalheiros/ktool/internal/kube"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ListPods(kc *kube.KubeClient, namespace string) ([]string, error) {
	cs, err := kc.Clientset()
	if err != nil {
		return nil, err
	}

	list, err := cs.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	names := make([]string, 0, len(list.Items))
	for _, pod := range list.Items {
		names = append(names, pod.Name)
	}
	sort.Strings(names)
	return names, nil
}

func PrintPods(kc *kube.KubeClient, pods []string, namespace string) {
	cs, err := kc.Clientset()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting clientset: %v\n", err)
		for _, p := range pods {
			fmt.Println(p)
		}
		return
	}

	list, err := cs.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		for _, p := range pods {
			fmt.Println(p)
		}
		return
	}

	podMap := make(map[string]corev1.Pod, len(list.Items))
	for _, pod := range list.Items {
		podMap[pod.Name] = pod
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tREADY\tSTATUS\tRESTARTS\tAGE")
	for _, name := range pods {
		pod, ok := podMap[name]
		if !ok {
			fmt.Fprintf(w, "%s\t-\t-\t-\t-\n", name)
			continue
		}

		ready := 0
		total := len(pod.Spec.Containers)
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.Ready {
				ready++
			}
		}

		restarts := int32(0)
		for _, cs := range pod.Status.ContainerStatuses {
			restarts += cs.RestartCount
		}

		age := time.Since(pod.CreationTimestamp.Time).Round(time.Second)
		fmt.Fprintf(w, "%s\t%d/%d\t%s\t%d\t%s\n",
			name, ready, total, string(pod.Status.Phase), restarts, formatAge(age))
	}
	w.Flush()
}

func StreamLogs(kc *kube.KubeClient, namespace, podName, container string, follow bool, tail int64, since string) error {
	cs, err := kc.Clientset()
	if err != nil {
		return err
	}

	opts := &corev1.PodLogOptions{
		Container: container,
		Follow:    follow,
	}

	if tail >= 0 {
		opts.TailLines = &tail
	}

	if since != "" {
		d, err := time.ParseDuration(since)
		if err != nil {
			return fmt.Errorf("invalid --since value %q: %w", since, err)
		}
		sinceSeconds := int64(d.Seconds())
		opts.SinceSeconds = &sinceSeconds
	}

	req := cs.CoreV1().Pods(namespace).GetLogs(podName, opts)
	stream, err := req.Stream(context.TODO())
	if err != nil {
		return fmt.Errorf("failed to open log stream for pod %q: %w", podName, err)
	}
	defer stream.Close()

	_, err = io.Copy(os.Stdout, stream)
	return err
}

func FindMatchingPods(kc *kube.KubeClient, namespace, query string) ([]string, error) {
	all, err := ListPods(kc, namespace)
	if err != nil {
		return nil, err
	}

	var matches []string
	for _, name := range all {
		if strings.HasPrefix(name, query) || strings.Contains(name, query) {
			matches = append(matches, name)
		}
	}
	return matches, nil
}

func SelectPodInteractive(pods []string) (string, error) {
	fzfCmd := exec.Command("fzf", "--height=40%", "--reverse", "--header=Select Pod", "--info=inline")
	fzfCmd.Stderr = os.Stderr
	fzfCmd.Stdin = strings.NewReader(strings.Join(pods, "\n"))

	var out bytes.Buffer
	fzfCmd.Stdout = &out

	if err := fzfCmd.Run(); err != nil {
		return "", nil
	}

	return strings.TrimSpace(out.String()), nil
}

func IsFzfInstalled() bool {
	_, err := exec.LookPath("fzf")
	return err == nil
}

func formatAge(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}
