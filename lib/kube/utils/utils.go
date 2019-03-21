package utils

import (
	"encoding/hex"

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

// GetKubeConfig returns kubernetes configuration
// from configPath file or, by default reads in-cluster configuration
func GetKubeConfig(configPath string) (*rest.Config, error) {
	// if path to kubeconfig was provided, init config from it
	if configPath != "" {
		return clientcmd.BuildConfigFromFlags("", configPath)
	}
	return rest.InClusterConfig()
}

// EncodeClusterName encodes cluster name for SNI matching
//
// For example:
//
// * Main cluster is main.example.com
// * Remote cluster is remote.example.com
//
// After 'tsh login' the URL of the Kubernetes endpoint of 'remote.example.com'
// when accessed 'via main.example.com' looks like this:
//
// 'k72656d6f74652e6578616d706c652e636f6d0a.main.example.com'
//
// For this to work, users have to add this address in public_addr section of kubernetes service
// to include 'main.example.com' in X509 '*.main.example.com' domain name
//
// where part '72656d6f74652e6578616d706c652e636f6d0a' is a hex encoded remote.example.com
//
// It is hex encoded to allow wildcard matching to work. In DNS wildcard match
// include only one '.'
//
func EncodeClusterName(clusterName string) string {
	// k is to avoid first letter to be a number
	return "k" + hex.EncodeToString([]byte(clusterName))
}
