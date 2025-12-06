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
	"cmp"
	"context"
	"encoding/json"
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
	// extKubeClusterName is the name of the extension that contains the
	// Teleport Kube cluster name. Its associated value is a string.
	extKubeClusterName = "teleport.kube.name"

	// extTeleClusterName is the name of the extension that contains the
	// Teleport cluster name. Its associated value is a string.
	extTeleClusterName = "kubeconfig.teleport.dev/teleport-cluster-name"

	// extProfileName is the name of the extension that contains the name of the
	// profile that should be used to connect to the Kube cluster. Its
	// associated value is a string.
	extProfileName = "kubeconfig.teleport.dev/profile-name"

	// extPreviousSelectedContext is the name of the extension that signals a
	// context that was set as the current context before the Teleport client
	// set a different context as the current context. Its associated value is
	// written as a null and it's never read.
	extPreviousSelectedContext = "teleport-prev-selec-ctx"
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

	// TLSServerNameFunc, if set, returns the SNI used when connecting to a
	// given kube cluster in a given Teleport cluster.
	TLSServerNameFunc func(teleportClusterName, kubeClusterName string) string

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

func (v Values) TLSServerName(teleportClusterName, kubeClusterName string) string {
	if v.TLSServerNameFunc == nil {
		return ""
	}

	return v.TLSServerNameFunc(teleportClusterName, kubeClusterName)
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

	if v.Exec != nil {
		// Called from tsh, use the exec plugin model.
		envVars := make([]clientcmdapi.ExecEnvVar, 0, len(v.Exec.Env))
		if v.Exec.Env != nil {
			for name, value := range v.Exec.Env {
				envVars = append(envVars, clientcmdapi.ExecEnvVar{Name: name, Value: value})
			}
		}

		for _, c := range v.KubeClusters {
			contextName := ContextName(v.TeleportClusterName, c)
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
			config.AuthInfos[contextName] = authInfo

			setStringExtensionInAuthInfo(config.AuthInfos[contextName], extTeleClusterName, v.TeleportClusterName)
			setStringExtensionInAuthInfo(config.AuthInfos[contextName], extKubeClusterName, c)

			config.Clusters[contextName] = &clientcmdapi.Cluster{
				Server:                   v.ClusterAddr,
				CertificateAuthorityData: cas,
				TLSServerName:            v.TLSServerName(v.TeleportClusterName, c),
			}

			setStringExtensionInCluster(config.Clusters[contextName], extTeleClusterName, v.TeleportClusterName)
			setStringExtensionInCluster(config.Clusters[contextName], extKubeClusterName, c)

			// XXX(espadolini): this assumes (as just about everything in
			// tsh does) that the profile name is exactly the same as the
			// hostname of the Proxy web address, and it would be more correct
			// to update every callsite to also pass the profile name
			var profileName string
			if v.ProxyAddr != "" {
				p, err := utils.Host(v.ProxyAddr)
				if err != nil {
					return trace.Wrap(err)
				}
				profileName = p
			}
			if profileName != "" {
				setStringExtensionInCluster(config.Clusters[contextName], extProfileName, profileName)
				setStringExtensionInAuthInfo(config.AuthInfos[contextName], extProfileName, profileName)
			}

			clusterName := contextName
			authName := contextName
			setContext(config.Contexts, contextName, clusterName, authName, profileName, v.TeleportClusterName, c, v.Namespace)
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

		contextName := v.TeleportClusterName
		var kubeClusterName string
		if len(v.KubeClusters) == 1 {
			kubeClusterName = v.KubeClusters[0]
			contextName = ContextName(v.TeleportClusterName, kubeClusterName)
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

			config.Clusters[contextName] = &clientcmdapi.Cluster{
				Server:                   v.ClusterAddr,
				CertificateAuthorityData: cas,
				TLSServerName:            v.TLSServerName(v.TeleportClusterName, cmp.Or(kubeClusterName, v.TeleportClusterName)),
			}

			setStringExtensionInCluster(config.Clusters[contextName], extTeleClusterName, v.TeleportClusterName)
			setStringExtensionInAuthInfo(config.AuthInfos[contextName], extTeleClusterName, v.TeleportClusterName)

			if kubeClusterName != "" {
				setStringExtensionInCluster(config.Clusters[contextName], extKubeClusterName, kubeClusterName)
				setStringExtensionInAuthInfo(config.AuthInfos[contextName], extKubeClusterName, kubeClusterName)
			}

			var profileName string
			if v.ProxyAddr != "" {
				p, err := utils.Host(v.ProxyAddr)
				if err != nil {
					return trace.Wrap(err)
				}
				profileName = p
			}
			if profileName != "" {
				setStringExtensionInCluster(config.Clusters[contextName], extProfileName, profileName)
				setStringExtensionInAuthInfo(config.AuthInfos[contextName], extProfileName, profileName)
			}

			authName := contextName
			clusterName := contextName
			setContext(config.Contexts, contextName, clusterName, authName, profileName, v.TeleportClusterName, kubeClusterName, v.Namespace)
			setSelectedExtension(config.Contexts, config.CurrentContext, clusterName)
			config.CurrentContext = contextName
		} else if !trace.IsBadParameter(err) {
			return trace.Wrap(err)
		}
		log.WarnContext(context.Background(), "Kubernetes integration is not supported when logging in with a hardware private key", "error", err)
	}

	// kubeconfig files generated by older versions of Teleport might use a
	// single cluster with the same name as the Teleport cluster for all
	// contexts, and we can't unconditionally delete it because there might be
	// contexts that we haven't modified here that still use it (and it might in
	// fact be used by the single-cluster mode if there's no kube cluster name
	// available)
	if _, legacyClusterExists := config.Clusters[v.TeleportClusterName]; legacyClusterExists {
		var inUse bool
		for _, context := range config.Contexts {
			if context.Cluster == v.TeleportClusterName {
				inUse = true
				break
			}
		}
		if !inUse {
			delete(config.Clusters, v.TeleportClusterName)
		}
	}

	return SaveConfig(path, *config, fs)
}

func setContext(contexts map[string]*clientcmdapi.Context, name, cluster, auth, profileName, teleportClusterName, kubeClusterName, namespace string) {
	lastContext := contexts[name]
	newContext := &clientcmdapi.Context{
		Cluster:  cluster,
		AuthInfo: auth,
	}
	if lastContext != nil {
		newContext.Namespace = lastContext.Namespace
		newContext.Extensions = lastContext.Extensions
	}

	if teleportClusterName != "" {
		setStringExtensionInContext(newContext, extTeleClusterName, teleportClusterName)
	}
	if kubeClusterName != "" {
		setStringExtensionInContext(newContext, extKubeClusterName, kubeClusterName)
	}
	if profileName != "" {
		setStringExtensionInContext(newContext, extProfileName, profileName)
	}

	// If a user specifies the default namespace we should override it.
	// Otherwise we should carry the namespace previously defined for the context.
	if len(namespace) > 0 {
		newContext.Namespace = namespace
	}

	contexts[name] = newContext
}

// RemoveByProfileName removes Teleport configuration from the kubeconfig file
// for a given profile.
//
// If `path` is empty, RemoveByProfileName will try to guess it based on the
// environment or known defaults.
func RemoveByProfileName(path, profileName string) error {
	// Load existing kubeconfig from disk.
	config, err := Load(path)
	if err != nil {
		return trace.Wrap(err)
	}

	removeByProfileName(config, profileName)

	// Update kubeconfig on disk.
	return trace.Wrap(Save(path, *config))
}

func removeByProfileName(config *clientcmdapi.Config, profileName string) {
	maps.DeleteFunc(config.Clusters, func(k string, v *clientcmdapi.Cluster) bool {
		n, ok := getStringExtensionFromCluster(v, extProfileName)
		return ok && n == profileName
	})
	maps.DeleteFunc(config.AuthInfos, func(k string, v *clientcmdapi.AuthInfo) bool {
		n, ok := getStringExtensionFromAuthInfo(v, extProfileName)
		return ok && n == profileName
	})
	maps.DeleteFunc(config.Contexts, func(k string, v *clientcmdapi.Context) bool {
		n, ok := getStringExtensionFromContext(v, extProfileName)
		return ok && n == profileName
	})

	if config.CurrentContext != "" {
		if _, ok := config.Contexts[config.CurrentContext]; !ok {
			// we shouldn't leave a deleted context as the current context, so
			// we'll try restoring the context that was selected before we
			// started updating the file or we blank it
			config.CurrentContext = searchForSelectedCluster(config.Contexts)
		}
	}
}

// RemoveByServerAddr removes all clusters with the provided server address from
// kubeconfig, all contexts using said clusters, and all authinfos referenced by
// those clusters.
//
// If `path` is empty, RemoveByServerAddr will try to guess it based on the
// environment or known defaults.
func RemoveByServerAddr(path, serverAddr string) error {
	// Load existing kubeconfig from disk.
	config, err := Load(path)
	if err != nil {
		return trace.Wrap(err)
	}

	removeByServerAddr(config, serverAddr)

	// Update kubeconfig on disk.
	return trace.Wrap(Save(path, *config))
}

func removeByServerAddr(config *clientcmdapi.Config, serverAddr string) {
	for clusterName, cluster := range config.Clusters {
		if cluster.Server != serverAddr {
			continue
		}

		delete(config.Clusters, clusterName)

		for contextName, context := range config.Contexts {
			if context.Cluster != clusterName {
				continue
			}

			delete(config.AuthInfos, context.AuthInfo)
			delete(config.Contexts, contextName)
		}
	}

	if config.CurrentContext != "" {
		if _, ok := config.Contexts[config.CurrentContext]; !ok {
			// we shouldn't leave a deleted context as the current context, so
			// we'll try restoring the context that was selected before we
			// started updating the file or we blank it
			config.CurrentContext = searchForSelectedCluster(config.Contexts)
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

// ContextName returns a kubeconfig context name generated by this package if no
// template override is provided by the user.
func ContextName(teleportCluster, kubeCluster string) string {
	return teleportCluster + "-" + kubeCluster
}

// KubeClusterFromContext extracts the kubernetes cluster name from a kubeconfig
// context generated by this package.
func KubeClusterFromContext(contextName string, context *clientcmdapi.Context, teleportClusterName string) string {
	if context == nil {
		// we only have a context name available; assume the default format for
		// context names because we can't do much else
		if suffix, found := strings.CutPrefix(contextName, teleportClusterName+"-"); found {
			return suffix
		}
		return ""
	}

	extTeleClusterName, hasExtTeleCluster := getStringExtensionFromContext(context, extTeleClusterName)
	extKubeClusterName, hasExtKubeCluster := getStringExtensionFromContext(context, extKubeClusterName)

	if hasExtTeleCluster && hasExtKubeCluster && extTeleClusterName == teleportClusterName {
		return extKubeClusterName
	}

	// older kubeconfig files produced by this package used the Teleport cluster
	// name as the name of the cluster
	if context.Cluster == teleportClusterName {
		if hasExtKubeCluster {
			return extKubeClusterName
		}
		contextName, _ := strings.CutPrefix(contextName, teleportClusterName+"-")
		return contextName
	}

	return ""
}

func jsonMarshalString(s string) []byte {
	// this will encode invalid utf8 bytes as U+FFFD, but it is otherwise infallible
	j, _ := json.Marshal(s)
	return j
}

func getStringExtensionFromContext(context *clientcmdapi.Context, extName string) (string, bool) {
	if context == nil {
		return "", false
	}
	return getStringExtensionFromExtensionsMap(context.Extensions, extName)
}
func getStringExtensionFromCluster(cluster *clientcmdapi.Cluster, extName string) (string, bool) {
	if cluster == nil {
		return "", false
	}
	return getStringExtensionFromExtensionsMap(cluster.Extensions, extName)
}
func getStringExtensionFromAuthInfo(authInfo *clientcmdapi.AuthInfo, extName string) (string, bool) {
	if authInfo == nil {
		return "", false
	}
	return getStringExtensionFromExtensionsMap(authInfo.Extensions, extName)
}
func getStringExtensionFromExtensionsMap(extensions map[string]runtime.Object, extName string) (string, bool) {
	rawExt, _ := extensions[extName].(*runtime.Unknown)
	if rawExt == nil {
		return "", false
	}

	var s string
	if err := json.Unmarshal(rawExt.Raw, &s); err != nil {
		return "", false
	}

	return s, true
}

func setStringExtensionInContext(context *clientcmdapi.Context, extName, extVal string) {
	if context == nil {
		return
	}
	if context.Extensions == nil {
		context.Extensions = make(map[string]runtime.Object)
	}
	setStringExtensionInExtensionsMap(context.Extensions, extName, extVal)
}
func setStringExtensionInCluster(cluster *clientcmdapi.Cluster, extName, extVal string) {
	if cluster == nil {
		return
	}
	if cluster.Extensions == nil {
		cluster.Extensions = make(map[string]runtime.Object)
	}
	setStringExtensionInExtensionsMap(cluster.Extensions, extName, extVal)
}
func setStringExtensionInAuthInfo(authInfo *clientcmdapi.AuthInfo, extName, extVal string) {
	if authInfo == nil {
		return
	}
	if authInfo.Extensions == nil {
		authInfo.Extensions = make(map[string]runtime.Object)
	}
	setStringExtensionInExtensionsMap(authInfo.Extensions, extName, extVal)
}
func setStringExtensionInExtensionsMap(extensions map[string]runtime.Object, extName, extVal string) {
	if extensions == nil {
		return
	}
	extensions[extName] = &runtime.Unknown{
		Raw: jsonMarshalString(extVal),
	}
}

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
	if !ok || strings.HasPrefix(prevCluster, teleportCluster+"-") || prevCluster == "" {
		return
	}
	for _, v := range contexts {
		delete(v.Extensions, extPreviousSelectedContext)
	}
	if selected.Extensions == nil {
		selected.Extensions = make(map[string]runtime.Object)
	}
	selected.Extensions[extPreviousSelectedContext] = nil
}

// searchForSelectedCluster looks for contexts that were previously selected
// in order to restore the CurrentContext value.
// If no such key is found or multiple keys exist, it returns an empty selected
// cluster.
func searchForSelectedCluster(contexts map[string]*clientcmdapi.Context) string {
	count := 0
	selected := ""
	for k, v := range contexts {
		if _, ok := v.Extensions[extPreviousSelectedContext]; ok {
			delete(v.Extensions, extPreviousSelectedContext)
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
