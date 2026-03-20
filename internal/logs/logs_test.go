package logs

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aureliomalheiros/ktool/internal/kube"
)

const baseKubeconfig = `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: %s
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    user: test-user
    namespace: test-ns
  name: test-context
current-context: test-context
users:
- name: test-user
  user: {}
`

func newTestKubeClient(t *testing.T, serverURL string) *kube.KubeClient {
	t.Helper()
	content := fmt.Sprintf(baseKubeconfig, serverURL)
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write kubeconfig: %v", err)
	}
	t.Setenv("KUBECONFIG", path)

	kc, err := kube.NewKubeClient()
	if err != nil {
		t.Fatalf("failed to create KubeClient: %v", err)
	}
	return kc
}

func newFakePodServer(t *testing.T, pods []string) *httptest.Server {
	t.Helper()

	items := make([]string, 0, len(pods))
	for _, p := range pods {
		items = append(items, fmt.Sprintf(`{
			"metadata": {"name": %q, "creationTimestamp": "2024-01-01T00:00:00Z"},
			"spec": {"containers": [{"name": "main"}]},
			"status": {"phase": "Running", "containerStatuses": [{"ready": true, "restartCount": 0}]}
		}`, p))
	}

	body := fmt.Sprintf(
		`{"apiVersion":"v1","kind":"PodList","metadata":{"resourceVersion":"1"},"items":[%s]}`,
		strings.Join(items, ","),
	)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/namespaces/test-ns/pods", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, body)
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("failed to read pipe: %v", err)
	}
	r.Close()
	return buf.String()
}

func TestListPods(t *testing.T) {
	expected := []string{"pod-a", "pod-b", "pod-c"}
	server := newFakePodServer(t, expected)
	kc := newTestKubeClient(t, server.URL)

	got, err := ListPods(kc, "test-ns")
	if err != nil {
		t.Fatalf("ListPods failed: %v", err)
	}

	if len(got) != len(expected) {
		t.Fatalf("expected %d pods, got %d: %v", len(expected), len(got), got)
	}
	for i, name := range expected {
		if got[i] != name {
			t.Errorf("expected pod[%d] = %q, got %q", i, name, got[i])
		}
	}
}

func TestListPods_Sorted(t *testing.T) {
	server := newFakePodServer(t, []string{"zzz-pod", "aaa-pod", "mmm-pod"})
	kc := newTestKubeClient(t, server.URL)

	got, err := ListPods(kc, "test-ns")
	if err != nil {
		t.Fatalf("ListPods failed: %v", err)
	}

	want := []string{"aaa-pod", "mmm-pod", "zzz-pod"}
	for i, name := range want {
		if got[i] != name {
			t.Errorf("expected sorted pod[%d] = %q, got %q", i, name, got[i])
		}
	}
}

func TestListPods_ServerError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/namespaces/test-ns/pods", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	kc := newTestKubeClient(t, server.URL)

	_, err := ListPods(kc, "test-ns")
	if err == nil {
		t.Fatal("expected error on server error response, got nil")
	}
}

func TestStreamLogs(t *testing.T) {
	logContent := "line1\nline2\nline3\n"

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/namespaces/test-ns/pods/my-pod/log", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, logContent)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	kc := newTestKubeClient(t, server.URL)

	out := captureStdout(t, func() {
		if err := StreamLogs(kc, "test-ns", "my-pod", "", false, 50, ""); err != nil {
			t.Errorf("StreamLogs failed: %v", err)
		}
	})

	if !strings.Contains(out, "line1") || !strings.Contains(out, "line3") {
		t.Errorf("expected log content in output, got: %q", out)
	}
}

func newFakePodServerWithLogs(t *testing.T, podLogs map[string]string) *httptest.Server {
	t.Helper()
	names := make([]string, 0, len(podLogs))
	for p := range podLogs {
		names = append(names, p)
	}
	items := make([]string, 0, len(names))
	for _, p := range names {
		items = append(items, fmt.Sprintf(`{
			"metadata": {"name": %q, "creationTimestamp": "2024-01-01T00:00:00Z"},
			"spec": {"containers": [{"name": "main"}]},
			"status": {"phase": "Running", "containerStatuses": [{"ready": true, "restartCount": 0}]}
		}`, p))
	}
	listBody := fmt.Sprintf(
		`{"apiVersion":"v1","kind":"PodList","metadata":{"resourceVersion":"1"},"items":[%s]}`,
		strings.Join(items, ","),
	)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/namespaces/test-ns/pods", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, listBody)
	})
	for pod, content := range podLogs {
		path := "/api/v1/namespaces/test-ns/pods/" + pod + "/log"
		content := content
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprint(w, content)
		})
	}

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

func TestStreamLogsMulti(t *testing.T) {
	podLogs := map[string]string{
		"pod-a": "alpha-one\nalpha-two\n",
		"pod-b": "beta-one\n",
	}
	server := newFakePodServerWithLogs(t, podLogs)
	kc := newTestKubeClient(t, server.URL)

	pods := []string{"pod-a", "pod-b"}
	var buf bytes.Buffer
	err := StreamLogsMulti(context.Background(), kc, "test-ns", pods, "", false, 50, "", &buf)
	if err != nil {
		t.Fatalf("StreamLogsMulti failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "[pod-a] alpha-one") || !strings.Contains(out, "[pod-a] alpha-two") {
		t.Errorf("expected pod-a prefixed lines in output, got: %q", out)
	}
	if !strings.Contains(out, "[pod-b] beta-one") {
		t.Errorf("expected pod-b prefixed lines in output, got: %q", out)
	}
}

func TestStreamLogsMulti_EmptyPods(t *testing.T) {
	server := httptest.NewServer(http.NewServeMux())
	t.Cleanup(server.Close)
	kc := newTestKubeClient(t, server.URL)

	err := StreamLogsMulti(context.Background(), kc, "test-ns", nil, "", false, 50, "", io.Discard)
	if err != nil {
		t.Fatalf("expected nil error for empty pod list, got %v", err)
	}
}

func TestStreamLogs_InvalidSince(t *testing.T) {
	server := httptest.NewServer(http.NewServeMux())
	t.Cleanup(server.Close)

	kc := newTestKubeClient(t, server.URL)

	err := StreamLogs(kc, "test-ns", "my-pod", "", false, -1, "notaduration")
	if err == nil {
		t.Fatal("expected error for invalid --since value")
	}
	if !strings.Contains(err.Error(), "invalid --since") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestPrintPods_ContainsHeaders(t *testing.T) {
	server := newFakePodServer(t, []string{"my-pod"})
	kc := newTestKubeClient(t, server.URL)

	pods := []string{"my-pod"}
	out := captureStdout(t, func() {
		PrintPods(kc, pods, "test-ns")
	})

	if !strings.Contains(out, "NAME") {
		t.Error("expected output to contain NAME header")
	}
	if !strings.Contains(out, "STATUS") {
		t.Error("expected output to contain STATUS header")
	}
	if !strings.Contains(out, "my-pod") {
		t.Error("expected output to contain pod name")
	}
}

func TestFormatAge(t *testing.T) {
	tests := []struct {
		input time.Duration
		want  string
	}{
		{30 * time.Second, "30s"},
		{90 * time.Second, "1m"},
		{3600 * time.Second, "1h"},
		{86400 * time.Second, "1d"},
	}

	for _, tc := range tests {
		got := formatAge(tc.input)
		if got != tc.want {
			t.Errorf("formatAge(%v) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestIsFzfInstalled(t *testing.T) {
	_ = IsFzfInstalled()
}
