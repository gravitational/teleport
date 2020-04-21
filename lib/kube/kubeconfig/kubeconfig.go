// Package kubeconfig manages teleport entries in a local kubeconfig file.
package kubeconfig

import (
	"bytes"
	"os"
	"path/filepath"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"

	"github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

var log = logrus.WithFields(logrus.Fields{
	trace.Component: teleport.ComponentKubeClient,
})

// Values are Teleport user data needed to generate kubeconfig entries.
type Values struct {
	// Name is used to name kubeconfig sections ("context", "cluster" and
	// "user"). Should match Teleport cluster name.
	Name string
	// ClusterAddr is the public address the Kubernetes client will talk to,
	// usually a proxy.
	ClusterAddr string
	// Credentials are user credentials to use for authentication the
	// ClusterAddr. Only TLS fields (key/cert/CA) from Credentials are used.
	Credentials *client.Key
}

// UpdateWithClient adds Teleport configuration to kubeconfig based on the
// configured TeleportClient.
//
// If `path` is empty, UpdateWithClient will try to guess it based on the
// environment or known defaults.
func UpdateWithClient(path string, tc *client.TeleportClient) error {
	clusterAddr := tc.KubeClusterAddr()
	clusterName, _ := tc.KubeProxyHostPort()
	if tc.SiteName != "" {
		clusterName = tc.SiteName
	}
	creds, err := tc.LocalAgent().GetKey()
	if err != nil {
		return trace.Wrap(err)
	}

	return Update(path, Values{
		Name:        clusterName,
		ClusterAddr: clusterAddr,
		Credentials: creds,
	})
}

// Update adds Teleport configuration to kubeconfig.
//
// If `path` is empty, Update will try to guess it based on the environment or
// known defaults.
func Update(path string, v Values) error {
	config, err := load(path)
	if err != nil {
		return trace.Wrap(err)
	}

	cas := bytes.Join(v.Credentials.TLSCAs(), []byte("\n"))
	// Validate the provided credentials, to avoid partially-populated
	// kubeconfig.
	if len(v.Credentials.Priv) == 0 {
		return trace.BadParameter("private key missing in provided credentials")
	}
	if len(v.Credentials.TLSCert) == 0 {
		return trace.BadParameter("TLS certificate missing in provided credentials")
	}
	if len(cas) == 0 {
		return trace.BadParameter("TLS trusted CAs missing in provided credentials")
	}

	config.AuthInfos[v.Name] = &clientcmdapi.AuthInfo{
		ClientCertificateData: v.Credentials.TLSCert,
		ClientKeyData:         v.Credentials.Priv,
	}
	config.Clusters[v.Name] = &clientcmdapi.Cluster{
		Server:                   v.ClusterAddr,
		CertificateAuthorityData: cas,
	}

	lastContext := config.Contexts[v.Name]
	newContext := &clientcmdapi.Context{
		Cluster:  v.Name,
		AuthInfo: v.Name,
	}
	if lastContext != nil {
		newContext.Namespace = lastContext.Namespace
		newContext.Extensions = lastContext.Extensions
	}
	config.Contexts[v.Name] = newContext
	config.CurrentContext = v.Name
	return save(path, *config)
}

// Remove removes Teleport configuration from kubeconfig.
//
// If `path` is empty, Remove will try to guess it based on the environment or
// known defaults.
func Remove(path, name string) error {
	// Load existing kubeconfig from disk.
	config, err := load(path)
	if err != nil {
		return trace.Wrap(err)
	}

	// Remove Teleport related AuthInfos, Clusters, and Contexts from kubeconfig.
	delete(config.AuthInfos, name)
	delete(config.Clusters, name)
	delete(config.Contexts, name)

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
	return save(path, *config)
}

// load tries to read a kubeconfig file and if it can't, returns an error.
// One exception, missing files result in empty configs, not an error.
func load(path string) (*clientcmdapi.Config, error) {
	filename, err := finalPath(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	config, err := clientcmd.LoadFromFile(filename)
	if err != nil && !os.IsNotExist(err) {
		err = trace.ConvertSystemError(err)
		return nil, trace.WrapWithMessage(err, "failed to parse existing kubeconfig %q: %v", filename, err)
	}
	if config == nil {
		config = clientcmdapi.NewConfig()
	}

	return config, nil
}

// save saves updated config to location specified by environment variable or
// default location
func save(path string, config clientcmdapi.Config) error {
	filename, err := finalPath(path)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := clientcmd.WriteToFile(config, filename); err != nil {
		return trace.ConvertSystemError(err)
	}
	return nil
}

// finalPath returns the final path to kubeceonfig using, in order of
// precedence:
// - `customPath`, if not empty
// - ${KUBECONFIG} environment variable
// - ${HOME}/.kube/config
//
// finalPath also creates any parent directories for the returned path, if
// missing.
func finalPath(customPath string) (string, error) {
	if customPath == "" {
		customPath = pathFromEnv()
	}
	finalPath, err := utils.EnsureLocalPath(customPath, teleport.KubeConfigDir, teleport.KubeConfigFile)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return finalPath, nil
}

// pathFromEnv extracts location of kubeconfig from the environment.
func pathFromEnv() string {
	kubeconfig := os.Getenv(teleport.EnvKubeConfig)

	// The KUBECONFIG environment variable is a list. On Windows it's
	// semicolon-delimited. On Linux and macOS it's colon-delimited.
	parts := filepath.SplitList(kubeconfig)

	// Default behavior of kubectl is to return the first file from list.
	var configPath string
	if len(parts) > 0 {
		configPath = parts[0]
		log.Debugf("Using kubeconfig from environment: %q.", configPath)
	}

	return configPath
}
