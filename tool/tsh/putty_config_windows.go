//go:build windows

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

package main

import (
	"fmt"
	"net"
	"runtime"
	"sort"
	"strings"
	"syscall"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/exp/slices"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keypaths"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"
)

// the key should not include HKEY_CURRENT_USER
const puttyRegistryKey = `SOFTWARE\SimonTatham\PuTTY`
const puttyRegistrySessionsKey = puttyRegistryKey + `\Sessions`
const puttyRegistrySSHHostCAsKey = puttyRegistryKey + `\SshHostCAs`

// strings
const puttyProtocol = `ssh`
const puttyProxyTelnetCommand = `%tsh proxy ssh --cluster=%cluster --proxy=%proxyhost %user@%host:%port`

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
func addPuTTYSession(proxyHostname string, hostname string, port int, login string, ppkFilePath string, certificateFilePath string, commandToRun string, leafClusterName string) (bool, error) {
	puttySessionName := fmt.Sprintf(`%v%%20(proxy:%v)`, hostname, proxyHostname)
	if leafClusterName != "" {
		puttySessionName = fmt.Sprintf(`%v%%20(leaf:%v,proxy:%v)`, hostname, leafClusterName, proxyHostname)
	}
	registryKey := fmt.Sprintf(`%v\%v`, puttyRegistrySessionsKey, puttySessionName)
	sessionDwords := puttyRegistrySessionDwords{}
	sessionStrings := puttyRegistrySessionStrings{}

	// if the port passed is 0, this means "use server default" so we override it to 3022
	if port == 0 {
		port = puttyDefaultSSHPort
	}

	sessionDwords = puttyRegistrySessionDwords{
		Present:        puttyDwordPresent,
		PortNumber:     port,
		ProxyPort:      puttyDefaultProxyPort,
		ProxyMethod:    puttyDwordProxyMethod,
		ProxyLogToTerm: puttyDwordProxyLogToTerm,
	}

	sessionStrings = puttyRegistrySessionStrings{
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
	pk, err := getRegistryKey(registryKey)
	if err != nil {
		return false, trace.Wrap(err)
	}
	defer pk.Close()

	// write dwords
	if ok, err := registryWriteDword(pk, "Present", sessionDwords.Present); !ok {
		return false, trace.Wrap(err)
	}
	if ok, err := registryWriteDword(pk, "PortNumber", fmt.Sprintf("%v", sessionDwords.PortNumber)); !ok {
		return false, trace.Wrap(err)
	}
	if ok, err := registryWriteDword(pk, "ProxyPort", fmt.Sprintf("%v", sessionDwords.ProxyPort)); !ok {
		return false, trace.Wrap(err)
	}
	if ok, err := registryWriteDword(pk, "ProxyMethod", sessionDwords.ProxyMethod); !ok {
		return false, trace.Wrap(err)
	}
	if ok, err := registryWriteDword(pk, "ProxyLogToTerm", sessionDwords.ProxyLogToTerm); !ok {
		return false, trace.Wrap(err)
	}

	// write strings
	if ok, err := registryWriteString(pk, "Hostname", sessionStrings.Hostname); !ok {
		return false, trace.Wrap(err)
	}
	if ok, err := registryWriteString(pk, "Protocol", sessionStrings.Protocol); !ok {
		return false, trace.Wrap(err)
	}
	if ok, err := registryWriteString(pk, "ProxyHost", sessionStrings.ProxyHost); !ok {
		return false, trace.Wrap(err)
	}
	if ok, err := registryWriteString(pk, "ProxyUsername", sessionStrings.ProxyUsername); !ok {
		return false, trace.Wrap(err)
	}
	if ok, err := registryWriteString(pk, "ProxyTelnetCommand", sessionStrings.ProxyTelnetCommand); !ok {
		return false, trace.Wrap(err)
	}
	if ok, err := registryWriteString(pk, "PublicKeyFile", sessionStrings.PublicKeyFile); !ok {
		return false, trace.Wrap(err)
	}
	if ok, err := registryWriteString(pk, "DetachedCertificate", sessionStrings.DetachedCertificate); !ok {
		return false, trace.Wrap(err)
	}
	if ok, err := registryWriteString(pk, "UserName", sessionStrings.UserName); !ok {
		return false, trace.Wrap(err)
	}

	return true, nil
}

// addHostCAPublicKey adds a host CA to the registry with a set of space-separated hostnames
func addHostCAPublicKey(keyName string, publicKey string, hostname string) (bool, error) {
	registryKeyName := fmt.Sprintf(`%v\%v`, puttyRegistrySSHHostCAsKey, keyName)

	// get the subkey with the host CA key name
	registryKey, err := getRegistryKey(registryKeyName)
	if err != nil {
		return false, trace.Wrap(err)
	}
	hostList, _, err := registryKey.GetStringsValue("MatchHosts")
	if err != nil {
		// ERROR_FILE_NOT_FOUND is an acceptable error, meaning that the value does not already
		// exist and it must be created
		if err != syscall.ERROR_FILE_NOT_FOUND {
			log.Debugf("Can't get registry value %v: %T", registryKeyName, err)
			return false, trace.Wrap(err)
		}
	} else {
		// initialise empty hostlist if no value returned
		if len(hostList) == 0 {
			hostList = []string{}
		}
	}
	defer registryKey.Close()

	// iterate over the list of hostnames provided
	// if an FQDN is provided and there are already entries in the list, see whether it can be covered
	// by a wildcard hostname that already exists in the list and skip adding it.
	if len(hostList) > 0 && strings.Contains(hostname, ".") {
		fullHostname := strings.Split(hostname, ".")
		wildcardDomain := fmt.Sprintf("*.%s", strings.Join(fullHostname[1:], "."))
		if !slices.Contains(hostList, wildcardDomain) {
			log.Debugf("Adding wildcard %q to hostList", wildcardDomain)
			hostList = append(hostList, wildcardDomain)
			log.Debugf("Removing hostname %q from hostList as it's now covered by a wildcard", hostname)
			hostList = utils.RemoveFromSlice(hostList, hostname)
		} else {
			log.Debugf("Not adding %q because it's already covered by %q", hostname, wildcardDomain)
		}
	} else {
		if !slices.Contains(hostList, hostname) {
			log.Debugf("Adding %q to hostList", hostname)
			hostList = append(hostList, hostname)
		} else {
			log.Debugf("%q is already present in hostList", hostname)
		}
	}
	sort.Strings(hostList)

	// write strings to subkey
	if ok, err := registryWriteMultiString(registryKey, "MatchHosts", hostList); !ok {
		return false, trace.Wrap(err)
	}
	if ok, err := registryWriteString(registryKey, "PublicKey", publicKey); !ok {
		return false, trace.Wrap(err)
	}

	// write dwords for signature acceptance
	if ok, err := registryWriteDword(registryKey, "PermitRSASHA1", puttyPermitRSASHA1); !ok {
		return false, trace.Wrap(err)
	}
	if ok, err := registryWriteDword(registryKey, "PermitRSASHA256", puttyPermitRSASHA256); !ok {
		return false, trace.Wrap(err)
	}
	if ok, err := registryWriteDword(registryKey, "PermitRSASHA512", puttyPermitRSASHA512); !ok {
		return false, trace.Wrap(err)
	}

	return true, nil
}

// formatLocalCommandString replaces placeholders in a constant with actual values
func formatLocalCommandString(tshPath string, cluster string) string {
	// replace the placeholder "%cluster" with the actual cluster name as passed to the function
	clusterString := strings.ReplaceAll(puttyProxyTelnetCommand, "%cluster", cluster)
	// PuTTY needs its paths to be double-escaped i.e. C:\\Users\\User\\tsh.exe
	escapedTshPath := strings.ReplaceAll(tshPath, `\`, `\\`)
	return strings.ReplaceAll(clusterString, "%tsh", escapedTshPath)
}

// onPuttyConfig handles the `tsh config putty` subcommand
func onPuttyConfig(cf *CLIConf) error {
	if runtime.GOOS != constants.WindowsOS {
		return trace.NotImplemented("PuTTY config is only implemented on Windows")
	}

	tc, err := makeClient(cf, true)
	if err != nil {
		return trace.Wrap(err)
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

	// process leaf clusters if --leaf flag is passed
	if cf.LeafClusterName != "" {
		leafClusters, err := proxyClient.GetLeafClusters(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		// check whether we know about the leaf cluster named, error if not
		leafClustersNames := make([]string, 0, len(leafClusters))
		for _, leafCluster := range leafClusters {
			leafClustersNames = append(leafClustersNames, leafCluster.GetName())
		}
		if !slices.Contains(leafClustersNames, cf.LeafClusterName) {
			return trace.BadParameter("Cannot find registered leaf cluster %q. Use the leaf cluster name as it appears in the output of `tsh clusters`.", cf.LeafClusterName)
		}
		certificateFilePath = keypaths.SSHCertPath(keysDir, proxyHost, tc.Config.Username, cf.LeafClusterName)
	}

	// get all CAs for the cluster (including trusted clusters)
	var cas []types.CertAuthority
	err = tc.WithRootClusterClient(cf.Context, func(clt auth.ClientI) error {
		cas, err = clt.GetCertAuthorities(cf.Context, types.HostCA, false /* exportSecrets */)
		if err != nil {
			return trace.Wrap(err)
		}
		return nil
	})
	// iterate over CAs
	hostCAPublicKeys := make(map[string][]string)
	for _, ca := range cas {
		var publicKeys []string
		// if this is either the root or requested leaf cluster, process it
		if ca.GetName() == rootClusterName || ca.GetName() == cf.LeafClusterName {
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
				publicKeys = append(publicKeys, strings.TrimPrefix(strings.TrimSpace(string(ssh.MarshalAuthorizedKey(hostCABytes))), constants.SSHRSAType+" "))
				hostCAPublicKeys[ca.GetName()] = publicKeys
			}
		}
	}

	hostname := tc.Config.Host
	port := tc.Config.HostPort
	userHostString := hostname
	login := ""
	if tc.Config.HostLogin != "" {
		login = tc.Config.HostLogin
		userHostString = fmt.Sprintf("%v@%v", login, userHostString)
	}

	// add all host CA public keys for cluster
	for cluster, hostCAs := range hostCAPublicKeys {
		keyName := fmt.Sprintf(`TeleportHostCA-%v`, cluster)
		for i, publicKey := range hostCAs {
			// append indices to entries if we have multiple public keys for a CA
			if len(hostCAs) > 1 {
				keyName = fmt.Sprintf(`%v-%d`, keyName, i)
			}

			if ok, err := addHostCAPublicKey(keyName, publicKey, hostname); !ok {
				log.Errorf("Failed to add host CA key for %v: %T", cluster, err)
				return trace.Wrap(err)
			} else {
				log.Debugf("Added/updated host CA key %d for %v", i, cluster)
			}
		}
	}

	proxyCommandClusterName := rootClusterName
	if cf.LeafClusterName != "" {
		proxyCommandClusterName = cf.LeafClusterName
	}

	// format local command string (to run 'tsh proxy ssh')
	localCommandString := formatLocalCommandString(cf.executablePath, proxyCommandClusterName)

	// add session to registry
	if ok, err := addPuTTYSession(proxyHost, tc.Config.Host, port, login, ppkFilePath, certificateFilePath, localCommandString, cf.LeafClusterName); !ok {
		log.Errorf("Failed to add PuTTY session for %v: %T\n", userHostString, err)
		return trace.Wrap(err)
	} else {
		if cf.LeafClusterName != "" {
			fmt.Printf("Added PuTTY session for %v [leaf:%v,proxy:%v]\n", userHostString, cf.LeafClusterName, proxyHost)
		} else {
			fmt.Printf("Added PuTTY session for %v [proxy:%v]\n", userHostString, proxyHost)
		}
	}

	return nil
}
