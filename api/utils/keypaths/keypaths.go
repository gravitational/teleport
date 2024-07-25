/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package keypaths defines several keypaths used by multiple Teleport services.
package keypaths

import (
	"fmt"
	"path/filepath"
	"strings"
)

// keypath constants aren't exported in order to force
// helper function usage and maintain consistency.
const (
	// sessionKeyDir is a sub-directory where session keys are stored
	sessionKeyDir = "keys"
	// sshDirSuffix is the suffix of a sub-directory where SSH certificates are stored.
	sshDirSuffix = "-ssh"
	// fileNameKnownHosts is a file where known hosts are stored.
	fileNameKnownHosts = "known_hosts"
	// fileExtTLSCertLegacy is the legacy suffix/extension of a file where a TLS cert is stored.
	fileExtTLSCertLegacy = "-x509.pem"
	// fileExtTLSCert is the suffix/extension of a file where a TLS cert is stored.
	fileExtTLSCert = ".crt"
	// fileExtTLSKey is the suffix/extension of a file where a TLS private key is stored.
	fileExtTLSKey = ".key"
	// fileNameTLSCerts is a file where TLS Cert Authorities are stored.
	fileNameTLSCerts = "certs.pem"
	// fileExtCert is the suffix/extension of a file where an SSH Cert is stored.
	fileExtSSHCert = "-cert.pub"
	// fileExtPPK is the suffix/extension of a file where an SSH keypair is stored in PuTTY PPK format.
	fileExtPPK = ".ppk"
	// fileExtPub is the extension of a file where a public key is stored.
	fileExtPub = ".pub"
	// fileExtLocalCA is the extension of a file where a self-signed localhost CA cert is stored.
	fileExtLocalCA = "-localca.pem"
	// appDirSuffix is the suffix of a sub-directory where app TLS certs are stored.
	appDirSuffix = "-app"
	// db DirSuffix is the suffix of a sub-directory where db TLS certs are stored.
	dbDirSuffix = "-db"
	// kubeDirSuffix is the suffix of a sub-directory where kube TLS certs are stored.
	kubeDirSuffix = "-kube"
	// kubeConfigSuffix is the suffix of a kubeconfig file stored under the keys directory.
	kubeConfigSuffix = "-kubeconfig"
	// fileNameKubeCredLock is file name of lockfile used to prevent excessive login attempts.
	fileNameKubeCredLock = "kube_credentials.lock"
	// casDir is the directory name for where clusters certs are stored.
	casDir = "cas"
	// fileExtPem is the extension of a file where a public certificate is stored.
	fileExtPem = ".pem"
	// currentProfileFileName is a file containing the name of the current profile
	currentProfileFilename = "current-profile"
	// profileFileExt is the suffix of a profile file.
	profileFileExt = ".yaml"
	// oracleWalletDirSuffix is the suffix of the oracle wallet database directory.
	oracleWalletDirSuffix = "-wallet"
)

// Here's the file layout of all these keypaths.
// ~/.tsh/							   --> default base directory
// ├── current-profile                 --> file containing the name of the currently active profile
// ├── one.example.com.yaml            --> file containing profile details for proxy "one.example.com"
// ├── two.example.com.yaml            --> file containing profile details for proxy "two.example.com"
// ├── known_hosts                     --> trusted certificate authorities (their keys) in a format similar to known_hosts
// └── keys							   --> session keys directory
//    ├── one.example.com              --> Proxy hostname
//    │   ├── certs.pem                --> TLS CA certs for the Teleport CA
//    │   ├── foo                      --> Private Key for user "foo"
//    │   ├── foo.pub                  --> Public Key
//    │   ├── foo.ppk                  --> PuTTY PPK-formatted keypair for user "foo"
//    │   ├── kube_credentials.lock    --> Kube credential lockfile, used to prevent excessive relogin attempts
//    │   ├── foo-x509.pem             --> TLS client certificate for Auth Server
//    │   ├── foo-ssh                  --> SSH certs for user "foo"
//    │   │   ├── root-cert.pub        --> SSH cert for Teleport cluster "root"
//    │   │   └── leaf-cert.pub        --> SSH cert for Teleport cluster "leaf"
//    │   ├── foo-app                  --> App access certs for user "foo"
//    │   │   ├── root                 --> App access certs for cluster "root"
//    │   │   │   ├── appA.crt         --> TLS cert for app service "appA"
//    │   │   │   ├── appA.key         --> private key for app service "appA"
//    │   │   │   ├── appB.crt         --> TLS cert for app service "appB"
//    │   │   │   ├── appB.key         --> private key for app service "appB"
//    │   │   │   └── appB-localca.pem --> Self-signed localhost CA cert for app service "appB"
//    │   │   └── leaf                 --> App access certs for cluster "leaf"
//    │   │       ├── appC.crt         --> TLS cert for app service "appC"
//    │   │       └── appC.key         --> private key for app service "appC"
//    │   ├── foo-db                   --> Database access certs for user "foo"
//    │   │   ├── root                 --> Database access certs for cluster "root"
//    │   │   │   ├── dbA-x509.pem     --> TLS cert for database service "dbA"
//    │   │   │   ├── dbB-x509.pem     --> TLS cert for database service "dbB"
//    │   │   │   └── dbC-wallet       --> Oracle Client wallet Configuration directory.
//    │   │   ├── leaf                 --> Database access certs for cluster "leaf"
//    │   │   │   └── dbC-x509.pem     --> TLS cert for database service "dbC"
//    │   │   └── proxy-localca.pem    --> Self-signed TLS Routing local proxy CA
//    │   ├── foo-kube                 --> Kubernetes certs for user "foo"
//    │   |    ├── root                 --> Kubernetes certs for Teleport cluster "root"
//    │   |    │   ├── kubeA-kubeconfig --> standalone kubeconfig for Kubernetes cluster "kubeA"
//    │   |    │   ├── kubeA-x509.pem   --> TLS cert for Kubernetes cluster "kubeA"
//    │   |    │   ├── kubeB-kubeconfig --> standalone kubeconfig for Kubernetes cluster "kubeB"
//    │   |    │   ├── kubeB-x509.pem   --> TLS cert for Kubernetes cluster "kubeB"
//    │   |    │   └── localca.pem      --> Self-signed localhost CA cert for Teleport cluster "root"
//    │   |    └── leaf                 --> Kubernetes certs for Teleport cluster "leaf"
//    │   |        ├── kubeC-kubeconfig --> standalone kubeconfig for Kubernetes cluster "kubeC"
//    │   |        └── kubeC-x509.pem   --> TLS cert for Kubernetes cluster "kubeC"
//    |   └── cas                       --> Trusted clusters certificates
//    |        ├── root.pem             --> TLS CA for teleport cluster "root"
//    |        ├── leaf1.pem            --> TLS CA for teleport cluster "leaf1"
//    |        └── leaf2.pem            --> TLS CA for teleport cluster "leaf2"
//    └── two.example.com			    --> Additional proxy host entries follow the same format
//		  ...

// KeyDir returns the path to the keys directory.
//
// <baseDir>/keys
func KeyDir(baseDir string) string {
	return filepath.Join(baseDir, sessionKeyDir)
}

// CurrentProfile returns the path to the current profile file.
//
// <baseDir>/current-profile
func CurrentProfileFilePath(baseDir string) string {
	return filepath.Join(baseDir, currentProfileFilename)
}

// ProfileFilePath returns the path to the profile file for the given profile.
//
// <baseDir>/<profileName>.yaml
func ProfileFilePath(baseDir, profileName string) string {
	return filepath.Join(baseDir, profileName+profileFileExt)
}

// KnownHostsPath returns the path to the known hosts file.
//
// <baseDir>/known_hosts
func KnownHostsPath(baseDir string) string {
	return filepath.Join(baseDir, fileNameKnownHosts)
}

// ProxyKeyDir returns the path to the proxy's keys directory.
//
// <baseDir>/keys/<proxy>
func ProxyKeyDir(baseDir, proxy string) string {
	return filepath.Join(KeyDir(baseDir), proxy)
}

// UserKeyPath returns the path to the users's private key
// for the given proxy.
//
// <baseDir>/keys/<proxy>/<username>.
func UserKeyPath(baseDir, proxy, username string) string {
	return filepath.Join(ProxyKeyDir(baseDir, proxy), username)
}

// TLSCertPath returns the path to the users's TLS certificate
// for the given proxy.
//
// <baseDir>/keys/<proxy>/<username>-x509.pem
func TLSCertPath(baseDir, proxy, username string) string {
	return filepath.Join(ProxyKeyDir(baseDir, proxy), username+fileExtTLSCertLegacy)
}

// PublicKeyPath returns the path to the users's public key
// for the given proxy.
//
// <baseDir>/keys/<proxy>/<username>.pub
func PublicKeyPath(baseDir, proxy, username string) string {
	return filepath.Join(ProxyKeyDir(baseDir, proxy), username+fileExtPub)
}

// CAsDir returns path to trusted clusters certificates directory.
//
// <baseDir>/keys/<proxy>/cas
func CAsDir(baseDir, proxy string) string {
	return filepath.Join(ProxyKeyDir(baseDir, proxy), casDir)
}

// TLSCAsPath returns the path to the users's TLS CA's certificates
// for the given proxy.
// <baseDir>/keys/<proxy>/certs.pem
// DELETE IN 10.0. Deprecated
func TLSCAsPath(baseDir, proxy string) string {
	return filepath.Join(ProxyKeyDir(baseDir, proxy), fileNameTLSCerts)
}

// TLSCAsPathCluster returns the path to the specified cluster's CA directory.
//
// <baseDir>/keys/<proxy>/cas/<cluster>.pem
func TLSCAsPathCluster(baseDir, proxy, cluster string) string {
	return filepath.Join(ProxyKeyDir(baseDir, proxy), casDir, cluster+fileExtPem)
}

// SSHDir returns the path to the user's SSH directory for the given proxy.
//
// <baseDir>/keys/<proxy>/<username>-ssh
func SSHDir(baseDir, proxy, username string) string {
	return filepath.Join(ProxyKeyDir(baseDir, proxy), username+sshDirSuffix)
}

// PPKFilePath returns the path to the user's PuTTY PPK-formatted keypair
// for the given proxy and cluster.
//
// <baseDir>/keys/<proxy>/<username>.ppk
func PPKFilePath(baseDir, proxy, username string) string {
	return filepath.Join(ProxyKeyDir(baseDir, proxy), username+fileExtPPK)
}

// SSHCertPath returns the path to the users's SSH certificate
// for the given proxy and cluster.
//
// <baseDir>/keys/<proxy>/<username>-ssh/<cluster>-cert.pub
func SSHCertPath(baseDir, proxy, username, cluster string) string {
	return filepath.Join(SSHDir(baseDir, proxy, username), cluster+fileExtSSHCert)
}

// AppDir returns the path to the user's app directory
// for the given proxy.
//
// <baseDir>/keys/<proxy>/<username>-app
func AppDir(baseDir, proxy, username string) string {
	return filepath.Join(ProxyKeyDir(baseDir, proxy), username+appDirSuffix)
}

// AppCredentialDir returns the path to the user's app credential directory for
// the given proxy and cluster.
//
// <baseDir>/keys/<proxy>/<username>-app/<cluster>
func AppCredentialDir(baseDir, proxy, username, cluster string) string {
	return filepath.Join(AppDir(baseDir, proxy, username), cluster)
}

// AppCertPath returns the path to the user's TLS certificate
// for the given proxy, cluster, and app.
//
// <baseDir>/keys/<proxy>/<username>-app/<cluster>/<appname>.crt
func AppCertPath(baseDir, proxy, username, cluster, appname string) string {
	return filepath.Join(AppCredentialDir(baseDir, proxy, username, cluster), appname+fileExtTLSCert)
}

// AppKeyPath returns the path to the user's private key for the given proxy,
// cluster, and app.
//
// <baseDir>/keys/<proxy>/<username>-app/<cluster>/<appname>.key
func AppKeyPath(baseDir, proxy, username, cluster, appname string) string {
	return filepath.Join(AppCredentialDir(baseDir, proxy, username, cluster), appname+fileExtTLSKey)
}

// AppLocalCAPath returns the path to a self-signed localhost CA for the given
// proxy, cluster, and app.
//
// <baseDir>/keys/<proxy>/<username>-app/<cluster>/<appname>-localca.pem
func AppLocalCAPath(baseDir, proxy, username, cluster, appname string) string {
	return filepath.Join(AppCredentialDir(baseDir, proxy, username, cluster), appname+fileExtLocalCA)
}

// DatabaseDir returns the path to the user's database directory
// for the given proxy.
//
// <baseDir>/keys/<proxy>/<username>-db
func DatabaseDir(baseDir, proxy, username string) string {
	return filepath.Join(ProxyKeyDir(baseDir, proxy), username+dbDirSuffix)
}

// DatabaseCertDir returns the path to the user's database cert directory
// for the given proxy and cluster.
//
// <baseDir>/keys/<proxy>/<username>-db/<cluster>
func DatabaseCertDir(baseDir, proxy, username, cluster string) string {
	return filepath.Join(DatabaseDir(baseDir, proxy, username), cluster)
}

// DatabaseCertPath returns the path to the user's TLS certificate
// for the given proxy, cluster, and database.
//
// <baseDir>/keys/<proxy>/<username>-db/<cluster>/<dbname>-x509.pem
func DatabaseCertPath(baseDir, proxy, username, cluster, dbname string) string {
	return filepath.Join(DatabaseCertDir(baseDir, proxy, username, cluster), dbname+fileExtTLSCertLegacy)
}

// DatabaseOracleWalletDirectory returns the path to the user's Oracle Wallet configuration directory.
// for the given proxy, cluster and database.
// <baseDir>/keys/<proxy>/<username>-db/<cluster>/dbname-wallet/
func DatabaseOracleWalletDirectory(baseDir, proxy, username, cluster, dbname string) string {
	return filepath.Join(DatabaseCertDir(baseDir, proxy, username, cluster), dbname+oracleWalletDirSuffix)
}

// KubeDir returns the path to the user's kube directory
// for the given proxy.
//
// <baseDir>/keys/<proxy>/<username>-kube
func KubeDir(baseDir, proxy, username string) string {
	return filepath.Join(ProxyKeyDir(baseDir, proxy), username+kubeDirSuffix)
}

// KubeCertDir returns the path to the user's kube cert directory
// for the given proxy and cluster.
//
// <baseDir>/keys/<proxy>/<username>-kube/<cluster>
func KubeCertDir(baseDir, proxy, username, cluster string) string {
	return filepath.Join(KubeDir(baseDir, proxy, username), cluster)
}

// KubeCertPath returns the path to the user's TLS certificate
// for the given proxy, cluster, and kube cluster.
//
// <baseDir>/keys/<proxy>/<username>-kube/<cluster>/<kubename>-x509.pem
func KubeCertPath(baseDir, proxy, username, cluster, kubename string) string {
	return filepath.Join(KubeCertDir(baseDir, proxy, username, cluster), kubename+fileExtTLSCertLegacy)
}

// KubeConfigPath returns the path to the user's standalone kubeconfig
// for the given proxy, cluster, and kube cluster.
//
// <baseDir>/keys/<proxy>/<username>-kube/<cluster>/<kubename>-kubeconfig
func KubeConfigPath(baseDir, proxy, username, cluster, kubename string) string {
	return filepath.Join(KubeCertDir(baseDir, proxy, username, cluster), kubename+kubeConfigSuffix)
}

// KubeCredLockfilePath returns the kube credentials lock file for given proxy
//
// <baseDir>/keys/<proxy>/kube_credentials.lock
func KubeCredLockfilePath(baseDir, proxy string) string {
	return filepath.Join(ProxyKeyDir(baseDir, proxy), fileNameKubeCredLock)
}

// IsProfileKubeConfigPath makes a best effort attempt to check if the given
// path is a profile specific kubeconfig path generated by this package.
func IsProfileKubeConfigPath(path string) (bool, error) {
	if path == "" {
		return false, nil
	}
	// Split path on sessionKeyDir since we can't do filepath.Match with baseDir
	splitPath := strings.Split(path, "/"+sessionKeyDir+"/")
	match := fmt.Sprintf("*/*%v/*/*%v", kubeDirSuffix, kubeConfigSuffix)
	return filepath.Match(match, splitPath[len(splitPath)-1])
}

// IdentitySSHCertPath returns the path to the identity file's SSH certificate.
//
// <identity-file-dir>/<path>-cert.pub
func IdentitySSHCertPath(path string) string {
	return path + fileExtSSHCert
}

// TrimCertPathSuffix returns the given path with any cert suffix/extension trimmed off.
func TrimCertPathSuffix(path string) string {
	trimmedPath := strings.TrimSuffix(path, fileExtTLSCertLegacy)
	trimmedPath = strings.TrimSuffix(trimmedPath, fileExtTLSCert)
	trimmedPath = strings.TrimSuffix(trimmedPath, fileExtSSHCert)
	return trimmedPath
}

// TrimKeyPathSuffix returns the given path with any key suffix/extension trimmed off.
func TrimKeyPathSuffix(path string) string {
	return strings.TrimSuffix(path, fileExtTLSKey)
}
