package logs

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/aureliomalheiros/ktool/internal/kube"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
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

func podLogOptions(container string, follow bool, tail int64, since string) (*corev1.PodLogOptions, error) {
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
			return nil, fmt.Errorf("invalid --since value %q: %w", since, err)
		}
		sinceSeconds := int64(d.Seconds())
		opts.SinceSeconds = &sinceSeconds
	}

	return opts, nil
}

func openPodLogStream(ctx context.Context, cs *kubernetes.Clientset, namespace, podName string, opts *corev1.PodLogOptions) (io.ReadCloser, error) {
	req := cs.CoreV1().Pods(namespace).GetLogs(podName, opts)
	return req.Stream(ctx)
}

func StreamLogs(kc *kube.KubeClient, namespace, podName, container string, follow bool, tail int64, since string) error {
	cs, err := kc.Clientset()
	if err != nil {
		return err
	}

	opts, err := podLogOptions(container, follow, tail, since)
	if err != nil {
		return err
	}

	stream, err := openPodLogStream(context.Background(), cs, namespace, podName, opts)
	if err != nil {
		return fmt.Errorf("failed to open log stream for pod %q: %w", podName, err)
	}
	defer stream.Close()

	_, err = io.Copy(os.Stdout, stream)
	return err
}

func StreamLogsMulti(ctx context.Context, kc *kube.KubeClient, namespace string, podNames []string, container string, follow bool, tail int64, since string, out io.Writer) error {
	if len(podNames) == 0 {
		return nil
	}

	cs, err := kc.Clientset()
	if err != nil {
		return err
	}

	opts, err := podLogOptions(container, follow, tail, since)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	var writeMu sync.Mutex
	var errMu sync.Mutex
	var firstErr error

	setErr := func(e error) {
		errMu.Lock()
		defer errMu.Unlock()
		if firstErr == nil {
			firstErr = e
		}
	}

	for _, name := range podNames {
		podName := name
		wg.Add(1)
		go func() {
			defer wg.Done()

			stream, err := openPodLogStream(ctx, cs, namespace, podName, opts)
			if err != nil {
				setErr(fmt.Errorf("failed to open log stream for pod %q: %w", podName, err))
				return
			}
			defer stream.Close()

			scanner := bufio.NewScanner(stream)
			buf := make([]byte, 64*1024)
			scanner.Buffer(buf, 1024*1024)

			for scanner.Scan() {
				select {
				case <-ctx.Done():
					return
				default:
				}
				line := scanner.Text()
				writeMu.Lock()
				_, werr := fmt.Fprintf(out, "[%s] %s\n", podName, line)
				writeMu.Unlock()
				if werr != nil {
					setErr(werr)
					return
				}
			}
			if err := scanner.Err(); err != nil {
				if ctx.Err() != nil {
					return
				}
				setErr(fmt.Errorf("reading logs for pod %q: %w", podName, err))
			}
		}()
	}

	wg.Wait()
	return firstErr
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
