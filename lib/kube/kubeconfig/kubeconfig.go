/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

// Package kubeconfig manages teleport entries in a local kubeconfig file.
package kubeconfig

import (
	"bytes"
	"context"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"github.com/gravitational/trace"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

var log = logutils.NewPackageLogger(teleport.ComponentKey, teleport.ComponentKubeClient)

const (
	// teleportKubeClusterNameExtension is the name of the extension that
	// contains the Teleport Kube cluster name.
	teleportKubeClusterNameExtension = "teleport.kube.name"
)

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
	Credentials *client.KeyRing
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

	// Impersonate allows to define the default impersonated user.
	// Must be a subset of kubernetes_users or the Teleport username
	// otherwise Teleport will deny the request.
	Impersonate string
	// ImpersonateGroups allows to define the default values for impersonated groups.
	// Must be a subset of kubernetes_groups otherwise Teleport will deny
	// the request.
	ImpersonateGroups []string
	// Namespace allows to define the default namespace value.
	Namespace string
	// KubeClusters is a list of kubernetes clusters to generate contexts for.
	KubeClusters []string
	// SelectCluster is the name of the kubernetes cluster to set in
	// current-context.
	SelectCluster string
	// OverrideContext is the name of the context or template used when adding a new cluster.
	// If empty, the context name will be generated from the {teleport-cluster}-{kube-cluster}.
	OverrideContext string
}

// ExecValues contain values for configuring tsh as an exec auth plugin in
// kubeconfig.
type ExecValues struct {
	// TshBinaryPath is a path to the tsh binary for use as exec plugin.
	TshBinaryPath string
	// TshBinaryInsecure defines whether to set the --insecure flag in the tsh
	// exec plugin arguments. This is used when the proxy doesn't have a
	// trusted TLS cert during login.
	TshBinaryInsecure bool
	// Env is a map of environment variables to forward.
	Env map[string]string
}

// ConfigFS is a simple filesystem abstraction to allow alternative file
// writing options when generating kube config files.
type ConfigFS interface {
	// WriteFile writes the given data to path `name`, using the specified
	// permissions if the file is new.
	WriteFile(name string, data []byte, perm os.FileMode) error

	ReadFile(name string) ([]byte, error)
}

// defaultConfigFS is a ConfigFS that is backed by the system filesystem
type defaultConfigFS struct{}

func (defaultConfigFS) WriteFile(name string, data []byte, perm os.FileMode) error {
	return os.WriteFile(name, data, perm)
}

func (defaultConfigFS) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}

// Update adds Teleport configuration to kubeconfig.
//
// If `path` is empty, Update will try to guess it based on the environment or
// known defaults.
func Update(path string, v Values, storeAllCAs bool) error {
	return UpdateConfig(path, v, storeAllCAs, defaultConfigFS{})
}

// UpdateConfig adds Teleport configuration to kubeconfig, reading and writing
// from the supplied ConfigFS
//
// If `path` is empty, Update will try to guess it based on the environment or
// known defaults.
func UpdateConfig(path string, v Values, storeAllCAs bool, fs ConfigFS) error {
	contextTmpl, err := parseContextOverrideTemplate(v.OverrideContext)
	if err != nil {
		return trace.Wrap(err)
	}

	config, err := LoadConfig(path, fs)
	if err != nil {
		return trace.Wrap(err)
	}

	var clusterCAs [][]byte
	if storeAllCAs {
		clusterCAs = v.Credentials.TLSCAs()
	} else {
		clusterCAs, err = v.Credentials.RootClusterCAs()
		if err != nil {
			return trace.Wrap(err)
		}
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

		for _, c := range v.KubeClusters {
			contextName := ContextName(v.TeleportClusterName, c)
			authName := contextName
			if contextTmpl != nil {
				if contextName, err = executeKubeContextTemplate(contextTmpl, v.TeleportClusterName, c); err != nil {
					return trace.Wrap(err)
				}
			}
			execArgs := []string{
				"kube", "credentials",
				fmt.Sprintf("--kube-cluster=%s", c),
				fmt.Sprintf("--teleport-cluster=%s", v.TeleportClusterName),
			}
			if v.ProxyAddr != "" {
				execArgs = append(execArgs, fmt.Sprintf("--proxy=%s", v.ProxyAddr))
			}
			authInfo := &clientcmdapi.AuthInfo{
				Impersonate:       v.Impersonate,
				ImpersonateGroups: v.ImpersonateGroups,
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

			setContext(config.Contexts, contextName, clusterName, authName, c, v.Namespace)
		}
		if v.SelectCluster != "" {
			contextName := ContextName(v.TeleportClusterName, v.SelectCluster)
			if contextTmpl != nil {
				if contextName, err = executeKubeContextTemplate(contextTmpl, v.TeleportClusterName, v.SelectCluster); err != nil {
					return trace.Wrap(err)
				}
			}
			if _, ok := config.Contexts[contextName]; !ok {
				return trace.BadParameter("can't switch kubeconfig context to cluster %q, run 'tsh kube ls' to see available clusters", v.SelectCluster)
			}
			setSelectedExtension(config.Contexts, config.CurrentContext, v.TeleportClusterName)
			config.CurrentContext = contextName
		}
	} else {
		// When using credentials, we only support specifying a single Kubernetes
		// cluster.
		// It is a limitation because the certificate embeds the cluster name, and
		// Teleport relies on it to forward requests to the correct cluster.
		if len(v.KubeClusters) > 1 {
			return trace.BadParameter("Multi-cluster mode not supported when using Credentials")
		}

		clusterName := v.TeleportClusterName
		contextName := clusterName
		var kubeClusterName string
		if len(v.KubeClusters) == 1 {
			kubeClusterName = v.KubeClusters[0]
			contextName = ContextName(clusterName, kubeClusterName)
		}

		// Called when generating an identity file, use plaintext credentials.
		//
		// Validate the provided credentials, to avoid partially-populated
		// kubeconfig.

		// TODO (Joerger): Create a custom k8s Auth Provider or Exec Provider to
		// use hardware private keys for kube credentials (if possible)
		keyPEM, err := v.Credentials.TLSPrivateKey.SoftwarePrivateKeyPEM()
		if err == nil {
			if len(v.Credentials.TLSCert) == 0 {
				return trace.BadParameter("TLS certificate missing in provided credentials")
			}

			config.AuthInfos[contextName] = &clientcmdapi.AuthInfo{
				ClientCertificateData: v.Credentials.TLSCert,
				ClientKeyData:         keyPEM,
			}
			setContext(config.Contexts, contextName, clusterName, contextName, kubeClusterName, v.Namespace)
			setSelectedExtension(config.Contexts, config.CurrentContext, clusterName)
			config.CurrentContext = contextName
		} else if !trace.IsBadParameter(err) {
			return trace.Wrap(err)
		}
		log.WarnContext(context.Background(), "Kubernetes integration is not supported when logging in with a hardware private key", "error", err)
	}

	return SaveConfig(path, *config, fs)
}

func setContext(contexts map[string]*clientcmdapi.Context, name, cluster, auth, kubeName, namespace string) {
	lastContext := contexts[name]
	newContext := &clientcmdapi.Context{
		Cluster:  cluster,
		AuthInfo: auth,
	}
	if lastContext != nil {
		newContext.Namespace = lastContext.Namespace
		newContext.Extensions = lastContext.Extensions
	}

	if newContext.Extensions == nil {
		newContext.Extensions = make(map[string]runtime.Object)
	}
	if kubeName != "" {
		newContext.Extensions[teleportKubeClusterNameExtension] = &runtime.Unknown{
			// We need to wrap the kubeName in quotes to make sure it is parsed as a string.
			Raw: []byte(fmt.Sprintf("%q", kubeName)),
		}
	}

	// If a user specifies the default namespace we should override it.
	// Otherwise we should carry the namespace previously defined for the context.
	if len(namespace) > 0 {
		newContext.Namespace = namespace
	}

	contexts[name] = newContext
}

// RemoveByClusterName removes Teleport configuration from kubeconfig.
//
// If `path` is empty, RemoveByClusterName will try to guess it based on the environment or
// known defaults.
func RemoveByClusterName(path, clusterName string) error {
	// Load existing kubeconfig from disk.
	config, err := Load(path)
	if err != nil {
		return trace.Wrap(err)
	}

	removeByClusterName(config, clusterName)

	// Update kubeconfig on disk.
	return trace.Wrap(Save(path, *config))
}

func removeByClusterName(config *clientcmdapi.Config, clusterName string) {
	// Remove Teleport related AuthInfos, Clusters, and Contexts from kubeconfig.
	maps.DeleteFunc(
		config.Contexts,
		func(key string, val *clientcmdapi.Context) bool {
			if !strings.HasPrefix(key, clusterName) && val.Cluster != clusterName {
				return false
			}
			delete(config.AuthInfos, val.AuthInfo)
			delete(config.Clusters, val.Cluster)
			return true
		},
	)
	prevSelectedCluster := searchForSelectedCluster(config.Contexts)
	// Take an element from the list of contexts and make it the current
	// context, unless current context points to something else.
	if strings.HasPrefix(config.CurrentContext, clusterName) {
		config.CurrentContext = prevSelectedCluster
	}
}

// RemoveByServerAddr removes all clusters with the provided server address
// from kubeconfig
//
// If `path` is empty, RemoveByServerAddr will try to guess it based on the
// environment or known defaults.
func RemoveByServerAddr(path, wantServer string) error {
	// Load existing kubeconfig from disk.
	config, err := Load(path)
	if err != nil {
		return trace.Wrap(err)
	}

	removeByServerAddr(config, wantServer)

	// Update kubeconfig on disk.
	return trace.Wrap(Save(path, *config))
}

func removeByServerAddr(config *clientcmdapi.Config, wantServer string) {
	for clusterName, cluster := range config.Clusters {
		if cluster.Server == wantServer {
			removeByClusterName(config, clusterName)
		}
	}
}

// Load tries to read a kubeconfig file and if it can't, returns an error.
// One exception, missing files result in empty configs, not an error.
func Load(path string) (*clientcmdapi.Config, error) {
	return LoadConfig(path, defaultConfigFS{})
}

// LoadConfig tries to read a kubeconfig file and if it can't, returns an error.
// One exception, missing files result in empty configs, not an error.
func LoadConfig(path string, fs ConfigFS) (*clientcmdapi.Config, error) {
	filename, err := finalPath(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	configBytes, err := fs.ReadFile(filename)
	switch {
	case os.IsNotExist(err):
		return clientcmdapi.NewConfig(), nil

	case err != nil:
		err = trace.ConvertSystemError(err)
		return nil, trace.WrapWithMessage(err, "failed to load existing kubeconfig %q: %v", filename, err)
	}

	config, err := clientcmd.Load(configBytes)
	if err != nil {
		err = trace.ConvertSystemError(err)
		return nil, trace.WrapWithMessage(err, "failed to parse existing kubeconfig %q: %v", filename, err)
	}
	if config == nil {
		config = clientcmdapi.NewConfig()
	}

	// Now that we are using clientcmd.Load() we need to manually set all of the
	// object origin values manually. We used to use clientcmd.LoadFile() that
	// did it for us.
	setConfigOriginsAndDefaults(config, filename)

	return config, nil
}

// setConfigOriginsAndDefaults sets up the origin info for the config file.
func setConfigOriginsAndDefaults(config *clientcmdapi.Config, filename string) {
	// set LocationOfOrigin on every Cluster, User, and Context
	for key, obj := range config.AuthInfos {
		obj.LocationOfOrigin = filename
		config.AuthInfos[key] = obj
	}
	for key, obj := range config.Clusters {
		obj.LocationOfOrigin = filename
		config.Clusters[key] = obj
	}
	for key, obj := range config.Contexts {
		obj.LocationOfOrigin = filename
		config.Contexts[key] = obj
	}

	if config.AuthInfos == nil {
		config.AuthInfos = map[string]*clientcmdapi.AuthInfo{}
	}
	if config.Clusters == nil {
		config.Clusters = map[string]*clientcmdapi.Cluster{}
	}
	if config.Contexts == nil {
		config.Contexts = map[string]*clientcmdapi.Context{}
	}
}

// Save saves updated config to location specified by environment variable or
// default location
func Save(path string, config clientcmdapi.Config) error {
	return SaveConfig(path, config, defaultConfigFS{})
}

// Save saves updated config to location specified by environment variable or
// default location.
func SaveConfig(path string, config clientcmdapi.Config, fs ConfigFS) error {
	filename, err := finalPath(path)
	if err != nil {
		return trace.Wrap(err)
	}

	configBytes, err := clientcmd.Write(config)
	if err != nil {
		return trace.ConvertSystemError(err)
	}

	if err := fs.WriteFile(filename, configBytes, 0600); err != nil {
		return trace.ConvertSystemError(err)
	}
	return nil

}

// finalPath returns the final path to kubeconfig using, in order of
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
		log.DebugContext(context.Background(), "Using kubeconfig from environment", "config_path", configPath)
	}

	return configPath
}

// ContextName returns a kubeconfig context name generated by this package.
func ContextName(teleportCluster, kubeCluster string) string {
	return fmt.Sprintf("%s-%s", teleportCluster, kubeCluster)
}

// KubeClusterFromContext extracts the kubernetes cluster name from context
// name generated by this package.
func KubeClusterFromContext(contextName string, ctx *clientcmdapi.Context, teleportCluster string) string {
	switch {
	// If the context name starts with teleport cluster name, it was
	// generated by tsh.
	case strings.HasPrefix(contextName, teleportCluster+"-"):
		return strings.TrimPrefix(contextName, teleportCluster+"-")
		// If the context cluster matches teleport cluster, it was generated by
		// tsh using --set-context-override flag.
	case ctx != nil && ctx.Cluster == teleportCluster:
		if v, ok := ctx.Extensions[teleportKubeClusterNameExtension]; ok {
			if raw, ok := v.(*runtime.Unknown); ok && trimQuotes(string(raw.Raw)) != "" {
				// The value is a JSON string, so we need to trim the quotes.
				return trimQuotes(string(raw.Raw))
			}
		}
		return contextName
	default:
		return ""
	}
}

func trimQuotes(s string) string {
	return strings.TrimSuffix(strings.TrimPrefix(s, "\""), "\"")
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
	setSelectedExtension(kc.Contexts, kc.CurrentContext, teleportCluster)
	kc.CurrentContext = kubeContext
	if err := Save("", *kc); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

const selectedExtension = "teleport-prev-selec-ctx"

// setSelectedExtension sets an extension to indentify that the current non-teleport
// context was selected before introducing Teleport contexts in kubeconfig.
// If the currentContext is not from Teleport, this function adds the following
// extensions:
//   - extension: null
//     name: teleport-prev-selec-ctx
//
// Only one context is allowed to have the selected extension. If other context has it,
// this function deletes it and introduces it in the desired context.
func setSelectedExtension(contexts map[string]*clientcmdapi.Context, prevCluster string, teleportCluster string) {
	selected, ok := contexts[prevCluster]
	if !ok || strings.HasPrefix(prevCluster, teleportCluster) || len(prevCluster) == 0 {
		return
	}
	for _, v := range contexts {
		delete(v.Extensions, selectedExtension)
	}

	selected.Extensions[selectedExtension] = nil
}

// searchForSelectedCluster looks for contexts that were previously selected
// in order to restore the CurrentContext value.
// If no such key is found or multiple keys exist, it returns an empty selected
// cluster.
func searchForSelectedCluster(contexts map[string]*clientcmdapi.Context) string {
	count := 0
	selected := ""
	for k, v := range contexts {
		if _, ok := v.Extensions[selectedExtension]; ok {
			delete(v.Extensions, selectedExtension)
			count++
			selected = k
		}
	}
	if count != 1 {
		return ""
	}
	return selected
}

// SelectedKubeCluster returns the Kubernetes cluster name of the default context
// if it belongs to the Teleport cluster provided.
func SelectedKubeCluster(path, teleportCluster string) (string, error) {
	kubeconfig, err := Load(path)
	if err != nil {
		return "", trace.Wrap(err)
	}

	if kubeCluster := KubeClusterFromContext(
		kubeconfig.CurrentContext,
		kubeconfig.Contexts[kubeconfig.CurrentContext],
		teleportCluster); kubeCluster != "" {
		return kubeCluster, nil
	}
	return "", trace.NotFound("default context does not belong to Teleport")
}
