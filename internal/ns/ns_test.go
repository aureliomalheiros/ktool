package ns

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

const noNamespaceKubeconfig = `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://localhost:6443
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    user: test-user
  name: test-context
current-context: test-context
users:
- name: test-user
  user: {}
`

const orphanContextKubeconfig = `apiVersion: v1
kind: Config
clusters: []
contexts: []
current-context: nonexistent-context
users: []
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

func newTestKubeClientWithServer(t *testing.T, serverURL string) *kube.KubeClient {
	t.Helper()
	content := fmt.Sprintf(baseKubeconfig, serverURL)
	return newTestKubeClient(t, content)
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

func newFakeNamespaceServer(t *testing.T, namespaces []string) *httptest.Server {
	t.Helper()

	items := make([]string, 0, len(namespaces))
	for _, ns := range namespaces {
		items = append(items, fmt.Sprintf(`{"metadata":{"name":%q}}`, ns))
	}

	body := fmt.Sprintf(
		`{"apiVersion":"v1","kind":"NamespaceList","metadata":{"resourceVersion":"1"},"items":[%s]}`,
		strings.Join(items, ","),
	)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/namespaces", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, body)
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

func TestCurrentNamespace(t *testing.T) {
	kc := newTestKubeClient(t, fmt.Sprintf(baseKubeconfig, "https://localhost:6443"))

	got := CurrentNamespace(kc)
	if got != "test-ns" {
		t.Errorf("expected %q, got %q", "test-ns", got)
	}
}

func TestCurrentNamespace_DefaultsWhenEmpty(t *testing.T) {
	kc := newTestKubeClient(t, noNamespaceKubeconfig)

	got := CurrentNamespace(kc)
	if got != "default" {
		t.Errorf("expected %q when namespace unset, got %q", "default", got)
	}
}

func TestSetNamespace(t *testing.T) {
	kc := newTestKubeClient(t, fmt.Sprintf(baseKubeconfig, "https://localhost:6443"))
	kubeconfigPath := kc.Path()

	if err := SetNamespace(kc, "new-ns"); err != nil {
		t.Fatalf("SetNamespace failed: %v", err)
	}

	t.Setenv("KUBECONFIG", kubeconfigPath)
	kc2, err := kube.NewKubeClient()
	if err != nil {
		t.Fatalf("failed to reload KubeClient: %v", err)
	}
	if got := CurrentNamespace(kc2); got != "new-ns" {
		t.Errorf("expected namespace %q after set, got %q", "new-ns", got)
	}
}

func TestSetNamespace_ContextNotFound(t *testing.T) {
	kc := newTestKubeClient(t, orphanContextKubeconfig)

	err := SetNamespace(kc, "some-ns")
	if err == nil {
		t.Fatal("expected error for missing context, got nil")
	}
	if !strings.Contains(err.Error(), "nonexistent-context") {
		t.Errorf("expected error to mention context name, got: %v", err)
	}
}

func TestSetNamespace_PreservesOtherFields(t *testing.T) {
	kc := newTestKubeClient(t, fmt.Sprintf(baseKubeconfig, "https://localhost:6443"))
	kubeconfigPath := kc.Path()

	if err := SetNamespace(kc, "changed-ns"); err != nil {
		t.Fatalf("SetNamespace failed: %v", err)
	}

	t.Setenv("KUBECONFIG", kubeconfigPath)
	kc2, err := kube.NewKubeClient()
	if err != nil {
		t.Fatalf("failed to reload KubeClient: %v", err)
	}

	cfg := kc2.Config()
	if cfg.CurrentContext != "test-context" {
		t.Errorf("CurrentContext should be preserved, got %q", cfg.CurrentContext)
	}
	ctx := cfg.Contexts["test-context"]
	if ctx.Cluster != "test-cluster" {
		t.Errorf("Cluster should be preserved, got %q", ctx.Cluster)
	}
}

func TestPrintNamespaces_ContainsActiveMarker(t *testing.T) {
	kc := newTestKubeClient(t, fmt.Sprintf(baseKubeconfig, "https://localhost:6443"))
	namespaces := []string{"default", "kube-system", "test-ns"}

	out := captureStdout(t, func() {
		PrintNamespaces(kc, namespaces)
	})

	if !strings.Contains(out, "*") {
		t.Error("expected output to contain '*' marker for active namespace")
	}
	if !strings.Contains(out, "test-ns") {
		t.Error("expected output to contain test-ns")
	}
	if !strings.Contains(out, "CURRENT") {
		t.Error("expected output to contain CURRENT header")
	}
}

func TestPrintNamespaces_ActiveNamespaceMarked(t *testing.T) {
	kc := newTestKubeClient(t, fmt.Sprintf(baseKubeconfig, "https://localhost:6443"))
	namespaces := []string{"default", "kube-system", "test-ns"}

	out := captureStdout(t, func() {
		PrintNamespaces(kc, namespaces)
	})

	lines := strings.Split(strings.TrimSpace(out), "\n")
	for _, line := range lines {
		if strings.Contains(line, "test-ns") {
			if !strings.Contains(line, "*") {
				t.Errorf("active namespace line should contain '*', got: %q", line)
			}
		}
		if strings.Contains(line, "default") || strings.Contains(line, "kube-system") {
			if strings.Contains(line, "*") {
				t.Errorf("inactive namespace line should not contain '*', got: %q", line)
			}
		}
	}
}

func TestListNamespaces(t *testing.T) {
	expected := []string{"default", "kube-system", "production"}
	server := newFakeNamespaceServer(t, expected)

	kc := newTestKubeClientWithServer(t, server.URL)

	got, err := ListNamespaces(kc)
	if err != nil {
		t.Fatalf("ListNamespaces failed: %v", err)
	}

	if len(got) != len(expected) {
		t.Fatalf("expected %d namespaces, got %d: %v", len(expected), len(got), got)
	}

	for i, name := range expected {
		if got[i] != name {
			t.Errorf("expected namespace[%d] = %q, got %q", i, name, got[i])
		}
	}
}

func TestListNamespaces_Sorted(t *testing.T) {
	server := newFakeNamespaceServer(t, []string{"zzz-ns", "aaa-ns", "mmm-ns"})
	kc := newTestKubeClientWithServer(t, server.URL)

	got, err := ListNamespaces(kc)
	if err != nil {
		t.Fatalf("ListNamespaces failed: %v", err)
	}

	want := []string{"aaa-ns", "mmm-ns", "zzz-ns"}
	for i, name := range want {
		if got[i] != name {
			t.Errorf("expected sorted namespace[%d] = %q, got %q", i, name, got[i])
		}
	}
}

func TestListNamespaces_ServerError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/namespaces", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	kc := newTestKubeClientWithServer(t, server.URL)

	_, err := ListNamespaces(kc)
	if err == nil {
		t.Fatal("expected error on server error response, got nil")
	}
}

func TestIsFzfInstalled(t *testing.T) {
	_ = IsFzfInstalled()
}
