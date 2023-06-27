/*
Copyright 2022 Gravitational, Inc.

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

package common

import (
	"fmt"
	"net"
	"strings"
	"syscall"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keypaths"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/puttyhosts"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils/registry"
)

// the key should not include HKEY_CURRENT_USER
const puttyRegistryKey = `SOFTWARE\SimonTatham\PuTTY`
const puttyRegistrySessionsKey = puttyRegistryKey + `\Sessions`
const puttyRegistrySSHHostCAsKey = puttyRegistryKey + `\SshHostCAs`

// strings
const puttyProtocol = `ssh`

// ints
const puttyDefaultSSHPort = 3022
const puttyDefaultProxyPort = 0 // no need to set the proxy port as it's abstracted by `tsh proxy ssh`

// dwords
const puttyDwordPresent = `00000001`
const puttyDwordProxyMethod = `00000005`    // run a local command
const puttyDwordProxyLogToTerm = `00000002` // only until session starts
const puttyPermitRSASHA1 = `00000000`
const puttyPermitRSASHA256 = `00000001`
const puttyPermitRSASHA512 = `00000001`

// despite the strings/ints in struct, these are stored in the registry as DWORDs
type puttyRegistrySessionDwords struct {
	Present        string // dword
	PortNumber     int    // dword
	ProxyPort      int    // dword
	ProxyMethod    string // dword
	ProxyLogToTerm string // dword
}

type puttyRegistrySessionStrings struct {
	Hostname            string
	Protocol            string
	ProxyHost           string
	ProxyUsername       string
	ProxyPassword       string
	ProxyTelnetCommand  string
	PublicKeyFile       string
	DetachedCertificate string
	UserName            string
}

// addPuTTYSession adds a PuTTY session for the given host/port to the Windows registry
func addPuTTYSession(proxyHostname string, hostname string, port int, login string, ppkFilePath string, certificateFilePath string, commandToRun string, leafClusterName string) error {
	// note: the use of ` and double % signs here is intentional
	// the registry key is named "hostname.example.com%20(proxy:teleport.example.com)"
	// this produces a session name which displays in PuTTY as "hostname.example.com (proxy:teleport.example.com)"
	puttySessionName := fmt.Sprintf(`%v%%20(proxy:%v)`, hostname, proxyHostname)
	if leafClusterName != "" {
		// the registry key is named "hostname.example.com%20(leaf:leaf.example.com,proxy:teleport.example.com)"
		// this produces a session name which displays in PuTTY as "hostname.example.com (leaf:leaf.example.com,proxy:teleport.example.com)"
		puttySessionName = fmt.Sprintf(`%v%%20(leaf:%v,proxy:%v)`, hostname, leafClusterName, proxyHostname)
	}
	registryKey := fmt.Sprintf(`%v\%v`, puttyRegistrySessionsKey, puttySessionName)

	// if the port passed is 0, this means "use server default" so we override it to 3022
	if port == 0 {
		port = puttyDefaultSSHPort
	}

	sessionDwords := puttyRegistrySessionDwords{
		Present:        puttyDwordPresent,
		PortNumber:     port,
		ProxyPort:      puttyDefaultProxyPort,
		ProxyMethod:    puttyDwordProxyMethod,
		ProxyLogToTerm: puttyDwordProxyLogToTerm,
	}

	sessionStrings := puttyRegistrySessionStrings{
		Hostname:            hostname,
		Protocol:            puttyProtocol,
		ProxyHost:           proxyHostname,
		ProxyUsername:       login,
		ProxyPassword:       "",
		ProxyTelnetCommand:  commandToRun,
		PublicKeyFile:       ppkFilePath,
		DetachedCertificate: certificateFilePath,
		UserName:            login,
	}

	// now check for and create the individual session key
	pk, err := registry.GetOrCreateRegistryKey(registryKey)
	if err != nil {
		return trace.Wrap(err)
	}
	defer pk.Close()

	// write dwords
	if err := registry.WriteDword(pk, "Present", sessionDwords.Present); err != nil {
		return trace.Wrap(err)
	}
	if err := registry.WriteDword(pk, "PortNumber", fmt.Sprintf("%v", sessionDwords.PortNumber)); err != nil {
		return trace.Wrap(err)
	}
	if err := registry.WriteDword(pk, "ProxyPort", fmt.Sprintf("%v", sessionDwords.ProxyPort)); err != nil {
		return trace.Wrap(err)
	}
	if err := registry.WriteDword(pk, "ProxyMethod", sessionDwords.ProxyMethod); err != nil {
		return trace.Wrap(err)
	}
	if err := registry.WriteDword(pk, "ProxyLogToTerm", sessionDwords.ProxyLogToTerm); err != nil {
		return trace.Wrap(err)
	}

	// write strings
	if err := registry.WriteString(pk, "Hostname", sessionStrings.Hostname); err != nil {
		return trace.Wrap(err)
	}
	if err := registry.WriteString(pk, "Protocol", sessionStrings.Protocol); err != nil {
		return trace.Wrap(err)
	}
	if err := registry.WriteString(pk, "ProxyHost", sessionStrings.ProxyHost); err != nil {
		return trace.Wrap(err)
	}
	if err := registry.WriteString(pk, "ProxyUsername", sessionStrings.ProxyUsername); err != nil {
		return trace.Wrap(err)
	}
	if err := registry.WriteString(pk, "ProxyTelnetCommand", sessionStrings.ProxyTelnetCommand); err != nil {
		return trace.Wrap(err)
	}
	if err := registry.WriteString(pk, "PublicKeyFile", sessionStrings.PublicKeyFile); err != nil {
		return trace.Wrap(err)
	}
	if err := registry.WriteString(pk, "DetachedCertificate", sessionStrings.DetachedCertificate); err != nil {
		return trace.Wrap(err)
	}
	if err := registry.WriteString(pk, "UserName", sessionStrings.UserName); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// addHostCAPublicKey adds a host CA to the registry with a set of space-separated hostnames
func addHostCAPublicKey(keyName string, publicKey string, hostname string) error {
	registryKeyName := fmt.Sprintf(`%v\%v`, puttyRegistrySSHHostCAsKey, keyName)

	// get the subkey with the host CA key name
	registryKey, err := registry.GetOrCreateRegistryKey(registryKeyName)
	if err != nil {
		return trace.Wrap(err)
	}
	defer registryKey.Close()
	hostList, _, err := registryKey.GetStringsValue("MatchHosts")
	if err != nil {
		// ERROR_FILE_NOT_FOUND is an acceptable error, meaning that the value does not already
		// exist and it must be created
		if err != syscall.ERROR_FILE_NOT_FOUND {
			log.Debugf("Can't get registry value %v: %T", registryKeyName, err)
			return trace.Wrap(err)
		}
	}
	// initialize an empty hostlist if there isn't one stored under the registry key
	if len(hostList) == 0 {
		hostList = []string{}
	}

	// add the new hostname to the existing hostList from the registry key (if one exists)
	hostList = puttyhosts.AddHostToHostList(hostList, hostname)

	// write strings to subkey
	if err := registry.WriteMultiString(registryKey, "MatchHosts", hostList); err != nil {
		return trace.Wrap(err)
	}
	if err := registry.WriteString(registryKey, "PublicKey", publicKey); err != nil {
		return trace.Wrap(err)
	}

	// write dwords for signature acceptance
	if err := registry.WriteDword(registryKey, "PermitRSASHA1", puttyPermitRSASHA1); err != nil {
		return trace.Wrap(err)
	}
	if err := registry.WriteDword(registryKey, "PermitRSASHA256", puttyPermitRSASHA256); err != nil {
		return trace.Wrap(err)
	}
	if err := registry.WriteDword(registryKey, "PermitRSASHA512", puttyPermitRSASHA512); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// onPuttyConfig handles the `tsh config putty` subcommand
func onPuttyConfig(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	// validate hostname against a naive regex to make sure it doesn't contain obviously illegal characters due
	// to typos or similar. setting an "invalid" key in the registry makes it impossible to delete via the PuTTY
	// UI and requires registry edits, so it's much better to error out early here.
	hostname := tc.Config.Host
	if !puttyhosts.NaivelyValidateHostname(hostname) {
		return trace.BadParameter("provided hostname %v does not look like a valid hostname. Make sure it doesn't contain illegal characters.", hostname)
	}

	port := tc.Config.HostPort
	userHostString := hostname
	login := ""
	if tc.Config.HostLogin != "" {
		login = tc.Config.HostLogin
		userHostString = fmt.Sprintf("%v@%v", login, userHostString)
	}

	// connect to proxy to fetch cluster info
	proxyClient, err := tc.ConnectToProxy(cf.Context)
	if err != nil {
		return trace.Wrap(err)
	}
	defer proxyClient.Close()

	// parse out proxy details
	proxyHost, _, err := net.SplitHostPort(tc.Config.SSHProxyAddr)
	if err != nil {
		return trace.Wrap(err)
	}

	// get root cluster name and set keypaths
	rootClusterName, err := proxyClient.RootClusterName(cf.Context)
	if err != nil {
		return trace.Wrap(err)
	}
	keysDir := profile.FullProfilePath(tc.Config.KeysDir)
	ppkFilePath := keypaths.PPKFilePath(keysDir, proxyHost, tc.Config.Username)
	certificateFilePath := keypaths.SSHCertPath(keysDir, proxyHost, tc.Config.Username, rootClusterName)

	// if a leaf cluster is requested, we need to check whether we can find it while running the CA loop below
	wantLeaf := cf.LeafClusterName != ""
	leafExists := false

	// get all CAs for the cluster (including trusted clusters)
	var cas []types.CertAuthority
	err = tc.WithRootClusterClient(cf.Context, func(clt auth.ClientI) error {
		cas, err = clt.GetCertAuthorities(cf.Context, types.HostCA, false /* exportSecrets */)
		if err != nil {
			return trace.Wrap(err)
		}
		return nil
	})
	if err != nil {
		return trace.Wrap(err)
	}
	// iterate over all the CAs
	hostCAPublicKeys := make(map[string][]string)
	for _, ca := range cas {
		// if this is either the root or the requested leaf cluster, process it
		if ca.GetName() == rootClusterName || (wantLeaf && ca.GetName() == cf.LeafClusterName) {
			// if we've found the requested leaf cluster, mark that we've found it
			if ca.GetName() == cf.LeafClusterName {
				leafExists = true
			}
			for i, key := range ca.GetTrustedSSHKeyPairs() {
				log.Debugf("%v CA [%v]: %v", ca.GetName(), i, key)
				kh, err := sshutils.MarshalKnownHost(sshutils.KnownHost{
					Hostname:      ca.GetClusterName(),
					AuthorizedKey: key.PublicKey,
				})
				if err != nil {
					return trace.Wrap(err)
				}
				_, _, hostCABytes, _, _, err := ssh.ParseKnownHosts([]byte(kh))
				if err != nil {
					return trace.Wrap(err)
				}

				hostCAPublicKey := strings.TrimPrefix(strings.TrimSpace(string(ssh.MarshalAuthorizedKey(hostCABytes))), constants.SSHRSAType+" ")
				hostCAPublicKeys[ca.GetName()] = append(hostCAPublicKeys[ca.GetName()], hostCAPublicKey)
			}
		}
	}
	// update the cert path if a leaf cluster was requested
	proxyCommandClusterName := rootClusterName
	if wantLeaf {
		// if we haven't found the requested leaf cluster, error out
		if !leafExists {
			return trace.NotFound("Cannot find registered leaf cluster %q. Use the leaf cluster name as it appears in the output of `tsh clusters`.", cf.LeafClusterName)
		}
		proxyCommandClusterName = cf.LeafClusterName
		certificateFilePath = keypaths.SSHCertPath(keysDir, proxyHost, tc.Config.Username, cf.LeafClusterName)
	}

	// add all host CA public keys for cluster
	for cluster, hostCAs := range hostCAPublicKeys {
		keyName := fmt.Sprintf(`TeleportHostCA-%v`, cluster)
		for i, publicKey := range hostCAs {
			// append indices to entries if we have multiple public keys for a CA
			if len(hostCAs) > 1 {
				keyName = fmt.Sprintf(`%v-%d`, keyName, i)
			}

			if err := addHostCAPublicKey(keyName, publicKey, hostname); err != nil {
				log.Errorf("Failed to add host CA key for %v: %T", cluster, err)
				return trace.Wrap(err)
			}
			log.Debugf("Added/updated host CA key %d for %v", i, cluster)
		}
	}

	// format local command string (to run 'tsh proxy ssh')
	localCommandString, err := puttyhosts.FormatLocalCommandString(cf.executablePath, proxyCommandClusterName)
	if err != nil {
		return trace.Wrap(err)
	}

	// add session to registry
	if err := addPuTTYSession(proxyHost, tc.Config.Host, port, login, ppkFilePath, certificateFilePath, localCommandString, cf.LeafClusterName); err != nil {
		log.Errorf("Failed to add PuTTY session for %v: %T\n", userHostString, err)
		return trace.Wrap(err)
	}

	// handle leaf clusters
	if wantLeaf {
		fmt.Printf("Added PuTTY session for %v [leaf:%v,proxy:%v]\n", userHostString, cf.LeafClusterName, proxyHost)
		return nil
	}

	fmt.Printf("Added PuTTY session for %v [proxy:%v]\n", userHostString, proxyHost)
	return nil
}
