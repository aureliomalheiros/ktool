package ns

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/aureliomalheiros/ktool/internal/kube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
)

func ListNamespaces(kc *kube.KubeClient) ([]string, error) {
	cs, err := kc.Clientset()
	if err != nil {
		return nil, err
	}

	list, err := cs.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list namespaces: %w", err)
	}

	names := make([]string, 0, len(list.Items))
	for _, ns := range list.Items {
		names = append(names, ns.Name)
	}
	sort.Strings(names)
	return names, nil
}

func CurrentNamespace(kc *kube.KubeClient) string {
	cfg := kc.Config()
	ctx, ok := cfg.Contexts[cfg.CurrentContext]
	if !ok || ctx.Namespace == "" {
		return "default"
	}
	return ctx.Namespace
}

func SetNamespace(kc *kube.KubeClient, namespace string) error {
	cfg := kc.Config()
	ctx, ok := cfg.Contexts[cfg.CurrentContext]
	if !ok {
		return fmt.Errorf("current context %q not found in kubeconfig", cfg.CurrentContext)
	}

	ctx.Namespace = namespace

	dir := filepath.Dir(kc.Path())
	tempFile, err := ioutil.TempFile(dir, "kubeconfig_tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tempPath := tempFile.Name()

	if err := clientcmd.WriteToFile(*cfg, tempPath); err != nil {
		tempFile.Close()
		os.Remove(tempPath)
		return fmt.Errorf("failed to write kubeconfig to temp file: %w", err)
	}
	tempFile.Close()

	backupPath := kc.Path() + ".bak"
	_ = copyFile(kc.Path(), backupPath)

	if err := os.Rename(tempPath, kc.Path()); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to replace kubeconfig: %w", err)
	}
	return nil
}

func PrintNamespaces(kc *kube.KubeClient, namespaces []string) {
	current := CurrentNamespace(kc)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "CURRENT\tNAME")
	for _, name := range namespaces {
		prefix := " "
		if name == current {
			prefix = "*"
		}
		fmt.Fprintf(w, "%s\t%s\n", prefix, name)
	}
	w.Flush()
}

func SelectNamespaceInteractive(kc *kube.KubeClient, namespaces []string) error {
	current := CurrentNamespace(kc)

	fzfCmd := exec.Command("fzf", "--height=40%", "--reverse", "--header=Select Namespace", "--info=inline")
	fzfCmd.Stderr = os.Stderr
	fzfCmd.Stdin = strings.NewReader(strings.Join(namespaces, "\n"))

	var out bytes.Buffer
	fzfCmd.Stdout = &out

	if err := fzfCmd.Run(); err != nil {
		return nil
	}

	selected := strings.TrimSpace(out.String())
	if selected == "" {
		return nil
	}

	if selected == current {
		fmt.Printf("Already on namespace \"%s\"\n", selected)
		return nil
	}

	if err := SetNamespace(kc, selected); err != nil {
		return err
	}
	fmt.Printf("Switched to namespace \"%s\"\n", selected)
	return nil
}

func IsFzfInstalled() bool {
	_, err := exec.LookPath("fzf")
	return err == nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	if _, err := io.Copy(out, in); err != nil {
		os.Remove(dst)
		return err
	}

	return out.Close()
}
