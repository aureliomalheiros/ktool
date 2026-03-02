package ctx

import (
	"bytes"
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
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

func ListContexts(kc *kube.KubeClient) (map[string]*api.Context, string) {
	cfg := kc.Config()
	return cfg.Contexts, cfg.CurrentContext
}

func CurrentContext(kc *kube.KubeClient) string {
	return kc.Config().CurrentContext
}

func SwitchContext(kc *kube.KubeClient, name string) error {
	cfg := kc.Config()
	if _, ok := cfg.Contexts[name]; !ok {
		return fmt.Errorf("context %s not found", name)
	}

	cfg.CurrentContext = name

	dir := filepath.Dir(kc.Path())
	tempFile, err := ioutil.TempFile(dir, "kubeconfig_tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tempPath := tempFile.Name()

	if err := clientcmd.WriteToFile(*kc.Config(), tempPath); err != nil {
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

func SelectContextInteractive(kc *kube.KubeClient) error {
	contexts, current := ListContexts(kc)

	var keys []string
	for k := range contexts {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	fzfCmd := exec.Command("fzf", "--height=40%", "--reverse", "--header=Select Context", "--info=inline")
	fzfCmd.Stderr = os.Stderr

	input := strings.Join(keys, "\n")
	fzfCmd.Stdin = strings.NewReader(input)

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
		fmt.Printf("Already on context \"%s\"\n", selected)
		return nil
	}

	if err := SwitchContext(kc, selected); err != nil {
		return err
	}
	fmt.Printf("Switched to context \"%s\"\n", selected)
	return nil
}

func PrintContexts(kc *kube.KubeClient) {
	contexts, current := ListContexts(kc)

	var keys []string
	for k := range contexts {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "CURRENT\tNAME\tCLUSTER\tAUTHINFO\tNAMESPACE")

	for _, name := range keys {
		c := contexts[name]
		prefix := " "
		if name == current {
			prefix = "*"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", prefix, name, c.Cluster, c.AuthInfo, c.Namespace)
	}
	w.Flush()
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
