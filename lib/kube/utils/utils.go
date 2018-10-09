package utils

import (
	"github.com/gravitational/trace"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// GetKubeClient returns instance of client to the kubernetes cluster
// using in-cluster configuration if available and falling back to
// configuration file under configPath otherwise
func GetKubeClient(configPath string) (client *kubernetes.Clientset, config *rest.Config, err error) {
	// if path to kubeconfig was provided, init config from it
	if configPath != "" {
		config, err = clientcmd.BuildConfigFromFlags("", configPath)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
	} else {
		// otherwise attempt to init as if connecting from cluster
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
	}

	client, err = kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return client, config, nil
}
