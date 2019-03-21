package client

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/client"
	kubeutils "github.com/gravitational/teleport/lib/kube/utils"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"

	"github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

var log = logrus.WithFields(logrus.Fields{
	trace.Component: teleport.ComponentKubeClient,
})

// UpdateKubeconfig adds Teleport configuration to kubeconfig.
func UpdateKubeconfig(tc *client.TeleportClient) error {
	config, err := LoadKubeConfig()
	if err != nil {
		return trace.Wrap(err)
	}

	clusterName, proxyPort := tc.KubeProxyHostPort()
	var clusterAddr string
	if tc.SiteName != "" {
		// In case of a remote cluster, use SNI subdomain to "point" to a remote cluster name
		clusterAddr = fmt.Sprintf("https://%v.%v:%v",
			kubeutils.EncodeClusterName(tc.SiteName), clusterName, proxyPort)
		clusterName = tc.SiteName
	} else {
		clusterAddr = fmt.Sprintf("https://%v:%v", clusterName, proxyPort)
	}

	creds, err := tc.LocalAgent().GetKey()
	if err != nil {
		return trace.Wrap(err)
	}
	certAuthorities, err := tc.LocalAgent().GetCertsPEM()
	if err != nil {
		return trace.Wrap(err)
	}

	config.AuthInfos[clusterName] = &clientcmdapi.AuthInfo{
		ClientCertificateData: creds.TLSCert,
		ClientKeyData:         creds.Priv,
	}
	config.Clusters[clusterName] = &clientcmdapi.Cluster{
		Server: clusterAddr,
		CertificateAuthorityData: certAuthorities,
	}

	lastContext := config.Contexts[clusterName]
	newContext := &clientcmdapi.Context{
		Cluster:  clusterName,
		AuthInfo: clusterName,
	}
	if lastContext != nil {
		newContext.Namespace = lastContext.Namespace
		newContext.Extensions = lastContext.Extensions
	}
	config.Contexts[clusterName] = newContext
	config.CurrentContext = clusterName
	return SaveKubeConfig(*config)
}

// RemoveKubeconifg removes Teleport configuration from kubeconfig.
func RemoveKubeconifg(tc *client.TeleportClient, clusterName string) error {
	// Load existing kubeconfig from disk.
	config, err := LoadKubeConfig()
	if err != nil {
		return trace.Wrap(err)
	}

	// Remove Teleport related AuthInfos, Clusters, and Contexts from kubeconfig.
	delete(config.AuthInfos, clusterName)
	delete(config.Clusters, clusterName)
	delete(config.Contexts, clusterName)

	// Take an element from the list of contexts and make it the current context.
	if len(config.Contexts) > 0 {
		var currentContext *clientcmdapi.Context
		for _, cc := range config.Contexts {
			currentContext = cc
			break
		}
		config.CurrentContext = currentContext.Cluster
	}

	// Update kubeconfig on disk.
	return SaveKubeConfig(*config)
}

// LoadKubeconfig tries to read a kubeconfig file and if it can't, returns an error.
// One exception, missing files result in empty configs, not an error.
func LoadKubeConfig() (*clientcmdapi.Config, error) {
	filename, err := utils.EnsureLocalPath(
		kubeconfigFromEnv(),
		teleport.KubeConfigDir,
		teleport.KubeConfigFile)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	config, err := clientcmd.LoadFromFile(filename)
	if err != nil && !os.IsNotExist(err) {
		return nil, trace.ConvertSystemError(err)
	}
	if config == nil {
		config = clientcmdapi.NewConfig()
	}

	return config, nil
}

// SaveKubeConfig saves updated config to location specified by environment variable or
// default location
func SaveKubeConfig(config clientcmdapi.Config) error {
	filename, err := utils.EnsureLocalPath(
		kubeconfigFromEnv(),
		teleport.KubeConfigDir,
		teleport.KubeConfigFile)
	if err != nil {
		return trace.Wrap(err)
	}

	err = clientcmd.WriteToFile(config, filename)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	return nil
}

// kubeconfigFromEnv extracts location of kubeconfig from the environment.
func kubeconfigFromEnv() string {
	kubeconfig := os.Getenv(teleport.EnvKubeConfig)

	// The KUBECONFIG environment variable is a list. On Windows it's
	// semicolon-delimited. On Linux and macOS it's colon-delimited.
	var parts []string
	switch runtime.GOOS {
	case teleport.WindowsOS:
		parts = strings.Split(kubeconfig, ";")
	default:
		parts = strings.Split(kubeconfig, ":")
	}

	// Default behavior of kubectl is to return the first file from list.
	var configpath string
	if len(parts) > 0 {
		configpath = parts[0]
	}
	log.Debugf("Found kubeconfig in environment at '%v'.", configpath)

	return configpath
}
