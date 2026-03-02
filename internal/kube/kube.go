package kube

import (
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

type KubeClient struct {
	config       *api.Config
	path         string
	clientConfig clientcmd.ClientConfig
}

func NewKubeClient() (*KubeClient, error) {
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get user home dir: %w", err)
		}
		kubeconfig = filepath.Join(home, ".kube", "config")
	}

	config, err := clientcmd.LoadFromFile(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig from %s: %w", kubeconfig, err)
	}

	clientConfig := clientcmd.NewNonInteractiveClientConfig(*config, config.CurrentContext, &clientcmd.ConfigOverrides{}, nil)

	return &KubeClient{
		config:       config,
		path:         kubeconfig,
		clientConfig: clientConfig,
	}, nil
}

// Config returns the underlying api.Config for advanced use. It is safe for read-only
// access; modifications should be followed by writing back via other helpers.
func (k *KubeClient) Config() *api.Config {
	return k.config
}

// Path returns the path to the kubeconfig file used by the client.
func (k *KubeClient) Path() string {
	return k.path
}
