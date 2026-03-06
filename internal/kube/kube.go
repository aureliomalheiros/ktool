package kube

import (
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
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

func (k *KubeClient) Config() *api.Config {
	return k.config
}

func (k *KubeClient) Path() string {
	return k.path
}

func (k *KubeClient) Clientset() (*kubernetes.Clientset, error) {
	restConfig, err := k.clientConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to build REST config: %w", err)
	}
	return kubernetes.NewForConfig(restConfig)
}
