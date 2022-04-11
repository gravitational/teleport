// Copyright 2021 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package kubeconfig manages teleport entries in a local kubeconfig file.
package kubeconfig

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	// TeleportClusterName is used to name kubeconfig sections ("context", "cluster" and
	// "user"). Should match Teleport cluster name.
	TeleportClusterName string
	// ClusterAddr is the public address the Kubernetes client will talk to,
	// usually a proxy.
	ClusterAddr string
	// Credentials are user credentials to use for authentication the
	// ClusterAddr. Only TLS fields (key/cert/CA) from Credentials are used.
	Credentials *client.Key
	// Exec contains optional values to use, when configuring tsh as an exec
	// auth plugin in kubeconfig.
	//
	// If not set, static key/cert from Credentials are written to kubeconfig
	// instead.
	Exec *ExecValues
	// ProxyAddr is the host:port address provided when running tsh kube login.
	// This value is empty if a proxy was not specified.
	ProxyAddr string

	// TLSServerName is SNI host value passed to the server.
	TLSServerName string
}

// ExecValues contain values for configuring tsh as an exec auth plugin in
// kubeconfig.
type ExecValues struct {
	// TshBinaryPath is a path to the tsh binary for use as exec plugin.
	TshBinaryPath string
	// KubeClusters is a list of kubernetes clusters to generate contexts for.
	KubeClusters []string
	// SelectCluster is the name of the kubernetes cluster to set in
	// current-context.
	SelectCluster string
	// TshBinaryInsecure defines whether to set the --insecure flag in the tsh
	// exec plugin arguments. This is used when the proxy doesn't have a
	// trusted TLS cert during login.
	TshBinaryInsecure bool
	// Env is a map of environment variables to forward.
	Env map[string]string
}

// Update adds Teleport configuration to kubeconfig.
//
// If `path` is empty, Update will try to guess it based on the environment or
// known defaults.
func Update(path string, v Values) error {
	config, err := Load(path)
	if err != nil {
		return trace.Wrap(err)
	}

	clusterCAs, err := v.Credentials.RootClusterCAs()
	if err != nil {
		return trace.Wrap(err)
	}

	cas := bytes.Join(clusterCAs, []byte("\n"))
	if len(cas) == 0 {
		return trace.BadParameter("TLS trusted CAs missing in provided credentials")
	}
	config.Clusters[v.TeleportClusterName] = &clientcmdapi.Cluster{
		Server:                   v.ClusterAddr,
		CertificateAuthorityData: cas,
		TLSServerName:            v.TLSServerName,
	}

	if v.Exec != nil {
		// Called from tsh, use the exec plugin model.
		clusterName := v.TeleportClusterName
		envVars := make([]clientcmdapi.ExecEnvVar, 0, len(v.Exec.Env))
		if v.Exec.Env != nil {
			for name, value := range v.Exec.Env {
				envVars = append(envVars, clientcmdapi.ExecEnvVar{Name: name, Value: value})
			}
		}

		for _, c := range v.Exec.KubeClusters {
			contextName := ContextName(v.TeleportClusterName, c)
			authName := contextName
			execArgs := []string{"kube", "credentials",
				fmt.Sprintf("--kube-cluster=%s", c),
				fmt.Sprintf("--teleport-cluster=%s", v.TeleportClusterName),
			}
			if v.ProxyAddr != "" {
				execArgs = append(execArgs, fmt.Sprintf("--proxy=%s", v.ProxyAddr))
			}
			authInfo := &clientcmdapi.AuthInfo{
				Exec: &clientcmdapi.ExecConfig{
					APIVersion: "client.authentication.k8s.io/v1beta1",
					Command:    v.Exec.TshBinaryPath,
					Args:       execArgs,
				},
			}
			if v.Exec.TshBinaryInsecure {
				authInfo.Exec.Args = append(authInfo.Exec.Args, "--insecure")
			}
			if len(envVars) > 0 {
				authInfo.Exec.Env = envVars
			}
			config.AuthInfos[authName] = authInfo

			setContext(config.Contexts, contextName, clusterName, authName)
		}
		if v.Exec.SelectCluster != "" {
			contextName := ContextName(v.TeleportClusterName, v.Exec.SelectCluster)
			if _, ok := config.Contexts[contextName]; !ok {
				return trace.BadParameter("can't switch kubeconfig context to cluster %q, run 'tsh kube ls' to see available clusters", v.Exec.SelectCluster)
			}
			config.CurrentContext = contextName
		}
	} else {
		// Called when generating an identity file, use plaintext credentials.
		//
		// Validate the provided credentials, to avoid partially-populated
		// kubeconfig.
		if len(v.Credentials.Priv) == 0 {
			return trace.BadParameter("private key missing in provided credentials")
		}
		if len(v.Credentials.TLSCert) == 0 {
			return trace.BadParameter("TLS certificate missing in provided credentials")
		}

		config.AuthInfos[v.TeleportClusterName] = &clientcmdapi.AuthInfo{
			ClientCertificateData: v.Credentials.TLSCert,
			ClientKeyData:         v.Credentials.Priv,
		}

		setContext(config.Contexts, v.TeleportClusterName, v.TeleportClusterName, v.TeleportClusterName)
		config.CurrentContext = v.TeleportClusterName
	}

	return Save(path, *config)
}

func setContext(contexts map[string]*clientcmdapi.Context, name, cluster, auth string) {
	lastContext := contexts[name]
	newContext := &clientcmdapi.Context{
		Cluster:  cluster,
		AuthInfo: auth,
	}
	if lastContext != nil {
		newContext.Namespace = lastContext.Namespace
		newContext.Extensions = lastContext.Extensions
	}
	contexts[name] = newContext
}

// Remove removes Teleport configuration from kubeconfig.
//
// If `path` is empty, Remove will try to guess it based on the environment or
// known defaults.
func Remove(path, name string) error {
	// Load existing kubeconfig from disk.
	config, err := Load(path)
	if err != nil {
		return trace.Wrap(err)
	}

	// Remove Teleport related AuthInfos, Clusters, and Contexts from kubeconfig.
	delete(config.AuthInfos, name)
	delete(config.Clusters, name)
	delete(config.Contexts, name)

	// Take an element from the list of contexts and make it the current
	// context, unless current context points to something else.
	if config.CurrentContext == name && len(config.Contexts) > 0 {
		for name := range config.Contexts {
			config.CurrentContext = name
			break
		}
	}

	// Update kubeconfig on disk.
	return Save(path, *config)
}

// Load tries to read a kubeconfig file and if it can't, returns an error.
// One exception, missing files result in empty configs, not an error.
func Load(path string) (*clientcmdapi.Config, error) {
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

// Save saves updated config to location specified by environment variable or
// default location
func Save(path string, config clientcmdapi.Config) error {
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
		customPath = PathFromEnv()
	}
	finalPath, err := utils.EnsureLocalPath(customPath, teleport.KubeConfigDir, teleport.KubeConfigFile)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return finalPath, nil
}

// PathFromEnv extracts location of kubeconfig from the environment.
func PathFromEnv() string {
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

// ContextName returns a kubeconfig context name generated by this package.
func ContextName(teleportCluster, kubeCluster string) string {
	return fmt.Sprintf("%s-%s", teleportCluster, kubeCluster)
}

// KubeClusterFromContext extracts the kubernetes cluster name from context
// name generated by this package.
func KubeClusterFromContext(contextName, teleportCluster string) string {
	// If context name doesn't start with teleport cluster name, it was not
	// generated by tsh.
	if !strings.HasPrefix(contextName, teleportCluster+"-") {
		return ""
	}
	return strings.TrimPrefix(contextName, teleportCluster+"-")
}

// SelectContext switches the active kubeconfig context to point to the
// provided kubeCluster in teleportCluster.
func SelectContext(teleportCluster, kubeCluster string) error {
	kc, err := Load("")
	if err != nil {
		return trace.Wrap(err)
	}

	kubeContext := ContextName(teleportCluster, kubeCluster)
	if _, ok := kc.Contexts[kubeContext]; !ok {
		return trace.NotFound("kubeconfig context %q not found", kubeContext)
	}
	kc.CurrentContext = kubeContext
	if err := Save("", *kc); err != nil {
		return trace.Wrap(err)
	}
	return nil
}
