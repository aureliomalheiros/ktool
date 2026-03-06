package kube

import (
	"os"
	"path/filepath"
	"testing"
)

const minimalKubeconfig = `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://localhost:6443
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

func writeTempKubeconfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write kubeconfig: %v", err)
	}
	return path
}

func TestNewKubeClient_WithKUBECONFIG(t *testing.T) {
	path := writeTempKubeconfig(t, minimalKubeconfig)
	t.Setenv("KUBECONFIG", path)

	kc, err := NewKubeClient()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if kc == nil {
		t.Fatal("expected non-nil KubeClient")
	}
}

func TestNewKubeClient_InvalidPath(t *testing.T) {
	t.Setenv("KUBECONFIG", "/nonexistent/path/config")

	_, err := NewKubeClient()
	if err == nil {
		t.Fatal("expected error for missing kubeconfig, got nil")
	}
}

func TestConfig(t *testing.T) {
	path := writeTempKubeconfig(t, minimalKubeconfig)
	t.Setenv("KUBECONFIG", path)

	kc, err := NewKubeClient()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg := kc.Config()
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.CurrentContext != "test-context" {
		t.Errorf("expected current context %q, got %q", "test-context", cfg.CurrentContext)
	}
}

func TestPath(t *testing.T) {
	path := writeTempKubeconfig(t, minimalKubeconfig)
	t.Setenv("KUBECONFIG", path)

	kc, err := NewKubeClient()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := kc.Path(); got != path {
		t.Errorf("expected path %q, got %q", path, got)
	}
}

func TestClientset(t *testing.T) {
	path := writeTempKubeconfig(t, minimalKubeconfig)
	t.Setenv("KUBECONFIG", path)

	kc, err := NewKubeClient()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cs, err := kc.Clientset()
	if err != nil {
		t.Fatalf("unexpected error building clientset: %v", err)
	}
	if cs == nil {
		t.Fatal("expected non-nil clientset")
	}
}

func TestNewKubeClient_DefaultsToHomeKubeconfig(t *testing.T) {
	t.Setenv("KUBECONFIG", "")

	home := t.TempDir()
	kubeDir := filepath.Join(home, ".kube")
	if err := os.MkdirAll(kubeDir, 0755); err != nil {
		t.Fatalf("failed to create .kube dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(kubeDir, "config"), []byte(minimalKubeconfig), 0600); err != nil {
		t.Fatalf("failed to write kubeconfig: %v", err)
	}
	t.Setenv("HOME", home)

	kc, err := NewKubeClient()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := filepath.Join(home, ".kube", "config")
	if kc.Path() != expected {
		t.Errorf("expected path %q, got %q", expected, kc.Path())
	}
}
