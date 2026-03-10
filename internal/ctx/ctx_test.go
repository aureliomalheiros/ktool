package ctx

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aureliomalheiros/ktool/internal/kube"
)

const multiContextKubeconfig = `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://localhost:6443
  name: cluster-a
- cluster:
    server: https://localhost:6444
  name: cluster-b
contexts:
- context:
    cluster: cluster-a
    user: user-a
    namespace: ns-a
  name: ctx-a
- context:
    cluster: cluster-b
    user: user-b
    namespace: ns-b
  name: ctx-b
current-context: ctx-a
users:
- name: user-a
  user: {}
- name: user-b
  user: {}
`

func newTestKubeClient(t *testing.T, content string) *kube.KubeClient {
	t.Helper()
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

func TestListContexts(t *testing.T) {
	kc := newTestKubeClient(t, multiContextKubeconfig)

	contexts, current := ListContexts(kc)

	if len(contexts) != 2 {
		t.Errorf("expected 2 contexts, got %d", len(contexts))
	}
	if _, ok := contexts["ctx-a"]; !ok {
		t.Error("expected context ctx-a to exist")
	}
	if _, ok := contexts["ctx-b"]; !ok {
		t.Error("expected context ctx-b to exist")
	}
	if current != "ctx-a" {
		t.Errorf("expected current context %q, got %q", "ctx-a", current)
	}
}

func TestCurrentContext(t *testing.T) {
	kc := newTestKubeClient(t, multiContextKubeconfig)

	got := CurrentContext(kc)
	if got != "ctx-a" {
		t.Errorf("expected %q, got %q", "ctx-a", got)
	}
}

func TestSwitchContext(t *testing.T) {
	kc := newTestKubeClient(t, multiContextKubeconfig)
	kubeconfigPath := kc.Path()

	if err := SwitchContext(kc, "ctx-b"); err != nil {
		t.Fatalf("SwitchContext failed: %v", err)
	}

	t.Setenv("KUBECONFIG", kubeconfigPath)
	kc2, err := kube.NewKubeClient()
	if err != nil {
		t.Fatalf("failed to reload KubeClient: %v", err)
	}
	if got := CurrentContext(kc2); got != "ctx-b" {
		t.Errorf("expected current context %q after switch, got %q", "ctx-b", got)
	}
}

func TestSwitchContext_NotFound(t *testing.T) {
	kc := newTestKubeClient(t, multiContextKubeconfig)

	err := SwitchContext(kc, "nonexistent-context")
	if err == nil {
		t.Fatal("expected error for nonexistent context, got nil")
	}
	if !strings.Contains(err.Error(), "nonexistent-context") {
		t.Errorf("expected error to mention context name, got: %v", err)
	}
}

func TestSwitchContext_SameContext(t *testing.T) {
	kc := newTestKubeClient(t, multiContextKubeconfig)
	kubeconfigPath := kc.Path()

	if err := SwitchContext(kc, "ctx-a"); err != nil {
		t.Fatalf("unexpected error switching to current context: %v", err)
	}

	t.Setenv("KUBECONFIG", kubeconfigPath)
	kc2, err := kube.NewKubeClient()
	if err != nil {
		t.Fatalf("failed to reload KubeClient: %v", err)
	}
	if got := CurrentContext(kc2); got != "ctx-a" {
		t.Errorf("expected current context %q, got %q", "ctx-a", got)
	}
}

func TestPrintContexts_ContainsActiveMarker(t *testing.T) {
	kc := newTestKubeClient(t, multiContextKubeconfig)

	out := captureStdout(t, func() {
		PrintContexts(kc)
	})

	if !strings.Contains(out, "*") {
		t.Error("expected output to contain '*' marker for active context")
	}
	if !strings.Contains(out, "ctx-a") {
		t.Error("expected output to contain ctx-a")
	}
	if !strings.Contains(out, "ctx-b") {
		t.Error("expected output to contain ctx-b")
	}
	if !strings.Contains(out, "CURRENT") {
		t.Error("expected output to contain header CURRENT")
	}
}

func TestPrintContexts_ActiveContextMarked(t *testing.T) {
	kc := newTestKubeClient(t, multiContextKubeconfig)

	out := captureStdout(t, func() {
		PrintContexts(kc)
	})

	lines := strings.Split(strings.TrimSpace(out), "\n")
	for _, line := range lines {
		if strings.Contains(line, "ctx-a") {
			if !strings.Contains(line, "*") {
				t.Errorf("active context line should contain '*', got: %q", line)
			}
		}
		if strings.Contains(line, "ctx-b") {
			if strings.Contains(line, "*") {
				t.Errorf("inactive context line should not contain '*', got: %q", line)
			}
		}
	}
}

func TestIsFzfInstalled(t *testing.T) {
	_ = IsFzfInstalled()
}
