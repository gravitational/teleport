package client

import (
	"fmt"
	"os"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// UpdateKubeconfig updates kubernetes kube config
func UpdateKubeconfig(tc *client.TeleportClient) error {
	config, err := LoadKubeConfig()
	if err != nil {
		return trace.Wrap(err)
	}
	clusterName := tc.ProxyHost()
	if tc.SiteName != "" && tc.SiteName != clusterName {
		clusterName = fmt.Sprintf("%v.%v", tc.SiteName, tc.ProxyHost())
	}
	clusterAddr := fmt.Sprintf("https://%v:%v", clusterName, tc.ProxyKubePort())

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

// LoadKubeconfig tries to read a kubeconfig file and if it can't, returns an error.
// One exception, missing files result in empty configs, not an error.
func LoadKubeConfig() (*clientcmdapi.Config, error) {
	filename, err := utils.EnsureLocalPath(
		os.Getenv(teleport.EnvKubeConfig), teleport.KubeConfigDir, teleport.KubeConfigFile)
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
		os.Getenv(teleport.EnvKubeConfig), teleport.KubeConfigDir, teleport.KubeConfigFile)
	if err != nil {
		return trace.Wrap(err)
	}
	err = clientcmd.WriteToFile(config, filename)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	return nil
}
