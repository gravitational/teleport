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

package common

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"syscall"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/utils/keypaths"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/puttyhosts"
	"github.com/gravitational/teleport/lib/utils/registry"
)

// the key should not include HKEY_CURRENT_USER
const puttyRegistryKey = `SOFTWARE\SimonTatham\PuTTY`
const puttyRegistrySessionsKey = puttyRegistryKey + `\Sessions`
const puttyRegistrySSHHostCAsKey = puttyRegistryKey + `\SshHostCAs`

// strings
const puttyProtocol = `ssh`

// ints
const puttyDefaultProxyPort = 0 // no need to set the proxy port as it's abstracted by `tsh proxy ssh`

// dwords
const puttyDwordPresent = `00000001`
const puttyDwordProxyMethod = `00000005`    // run a local command
const puttyDwordProxyLogToTerm = `00000002` // only until session starts
const puttyPermitRSASHA1 = `00000000`
const puttyPermitRSASHA256 = `00000001`
const puttyPermitRSASHA512 = `00000001`
const puttyAuthGSSAPI = `00000000`
const puttyAuthGSSAPIKEX = `00000000`

// despite the strings/ints in struct, these are stored in the registry as DWORDs
type puttyRegistrySessionDwords struct {
	Present        string // dword
	PortNumber     int    // dword
	ProxyPort      int    // dword
	ProxyMethod    string // dword
	ProxyLogToTerm string // dword
	AuthGSSAPI     string // dword
	AuthGSSAPIKEX  string // dword
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

	sessionDwords := puttyRegistrySessionDwords{
		Present:        puttyDwordPresent,
		PortNumber:     port,
		ProxyPort:      puttyDefaultProxyPort,
		ProxyMethod:    puttyDwordProxyMethod,
		ProxyLogToTerm: puttyDwordProxyLogToTerm,
		AuthGSSAPI:     puttyAuthGSSAPI,
		AuthGSSAPIKEX:  puttyAuthGSSAPIKEX,
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
	if err := registry.WriteDword(pk, "AuthGSSAPI", sessionDwords.AuthGSSAPI); err != nil {
		return trace.Wrap(err)
	}
	if err := registry.WriteDword(pk, "AuthGSSAPIKEX", sessionDwords.AuthGSSAPIKEX); err != nil {
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

// addHostCAPublicKey adds a host CA to the registry with a set of hostnames delimited by " || "
// as per PuTTY's "Validity" syntax.
func addHostCAPublicKey(registryHostCAStruct puttyhosts.HostCAPublicKeyForRegistry) error {
	ctx := context.TODO()
	registryKeyName := fmt.Sprintf(`%v\%v`, puttyRegistrySSHHostCAsKey, registryHostCAStruct.KeyName)

	// get the subkey with the host CA key name
	registryKey, err := registry.GetOrCreateRegistryKey(registryKeyName)
	if err != nil {
		return trace.Wrap(err)
	}
	defer registryKey.Close()

	// get the "new" string-based Validity value if present.
	validity, _, err := registryKey.GetStringValue("Validity")
	if err != nil {
		// ERROR_FILE_NOT_FOUND is an acceptable error, meaning that the value does not already
		// exist and it must be created
		if err != syscall.ERROR_FILE_NOT_FOUND {
			logger.DebugContext(ctx, "Can't get registry value",
				"registry_key", registryKeyName,
				"error", err,
			)
			return trace.Wrap(err)
		}
	}

	// split the Validity key out into a list of individual hostnames (hostList)
	hostList, err := puttyhosts.CheckAndSplitValidityKey(validity, registryHostCAStruct.KeyName)
	if err != nil {
		return trace.Wrap(err)
	}

	// get the "old" multistring-based MatchHosts value if present, so we can migrate it to the newer
	// "Validity" format and then delete it.
	matchHosts, _, err := registryKey.GetStringsValue("MatchHosts")
	if err != nil {
		// ERROR_FILE_NOT_FOUND is an acceptable error, meaning that the value does not already
		// exist and it must be created
		if err != syscall.ERROR_FILE_NOT_FOUND {
			logger.DebugContext(ctx, "Can't get registry value",
				"registry_key", registryKeyName,
				"error", err,
			)
			return trace.Wrap(err)
		}
	}
	// if matchHosts has any entries, we do a one-time migration of all the values from the "old" MatchHosts
	// multistring to the new Validity string,
	if len(matchHosts) > 0 {
		logger.DebugContext(ctx, "Found legacy MatchHosts value(s) in registry key, migrating to new Validity format",
			"match_host_count", len(matchHosts),
			"registry_key", registryKeyName,
		)
		hostList = append(hostList, matchHosts...)
	}

	// add the new hostname to the existing hostList from the registry key (if one exists)
	hostList = puttyhosts.AddHostToHostList(hostList, registryHostCAStruct.Hostname)

	// Reconstruct the "Validity" string using our hostList, separated by " || ".
	hostListValidity := strings.Join(hostList, " || ")

	// write strings to subkey
	// In beta versions of PuTTY 0.78 and the initial release of 'tsh puttyconfig', the list of valid hosts was
	// represented by a REG_MULTI_SZ called "MatchHosts". Newer versions of PuTTY, WinSCP and 'tsh puttyconfig' use
	// and prefer the string-formatted "Validity" instead. PuTTY will ignore "MatchHosts" when "Validity" is set.
	if err := registry.WriteString(registryKey, "Validity", hostListValidity); err != nil {
		return trace.Wrap(err)
	}
	if err := registry.WriteString(registryKey, "PublicKey", registryHostCAStruct.PublicKey); err != nil {
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

	// if matchHosts has any entries, delete the "MatchHosts" key from the registry as its entries were migrated above.
	if len(matchHosts) > 0 {
		logger.DebugContext(ctx, "Deleting legacy MatchHosts value(s) from registry key",
			"match_host_count", len(matchHosts),
			"registry_key", registryKeyName,
		)

		err := registryKey.DeleteValue("MatchHosts")
		// failure to delete this value isn't a fatal error, so we should continue regardless
		if err != nil {
			logger.DebugContext(ctx, "Failed to delete old MatchHosts value for key",
				"registry_key", registryHostCAStruct.KeyName,
				"error", err,
			)
		}
	}

	return nil
}

// onPuttyConfig handles the `tsh puttyconfig` subcommand
func onPuttyConfig(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	// connect to proxy to fetch cluster info
	clusterClient, err := tc.ConnectToCluster(cf.Context)
	if err != nil {
		return trace.Wrap(err)
	}
	defer clusterClient.Close()

	matches, err := tc.GetTargetNodes(cf.Context, clusterClient.AuthClient, client.SSHOptions{})
	if err != nil {
		return trace.Wrap(err)
	}

	switch len(matches) {
	case 0:
		return trace.NotFound("no matching hosts found")
	case 1:
		logger.DebugContext(cf.Context, "Using matched host", "host", matches[0])
	default:
		logger.DebugContext(cf.Context, "found multiple matching hosts", matches[0], matches[1])
		return trace.BadParameter("multiple matching hosts found")
	}

	// remove any spaces from the provided hostname. if the hostname contains a colon, it will be a
	// hostname:port combination so we split it. this is useful as shorthand when adding OpenSSH hosts
	// with `tsh puttyconfig user@host:22`, rather than using the longer `tsh puttyconfig --port 22 user@host`
	hostname := strings.TrimSpace(matches[0].Hostname)
	port := tc.Config.HostPort
	if splitHost, splitPort, err := net.SplitHostPort(hostname); err == nil {
		hostname = splitHost
		port, err = strconv.Atoi(splitPort)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	// validate the hostname against a naive regex to make sure it doesn't contain obviously illegal characters
	// due to typos or similar. setting an "invalid" key in the registry makes it impossible to delete via the
	// PuTTY UI and requires registry edits, so it's much better to error out early here.
	if !puttyhosts.NaivelyValidateHostname(hostname) {
		return trace.BadParameter("provided hostname %v does not look like a valid hostname. Make sure it doesn't contain illegal characters.", hostname)
	}

	userHostString := hostname
	login := ""
	if tc.Config.HostLogin != "" {
		login = strings.ReplaceAll(tc.Config.HostLogin, " ", "")
		userHostString = fmt.Sprintf("%v@%v", login, userHostString)
	}

	// parse out proxy details
	proxyHost, _, err := net.SplitHostPort(tc.Config.SSHProxyAddr)
	if err != nil {
		return trace.Wrap(err)
	}

	// get root cluster name and set keypaths
	rootClusterName := clusterClient.RootClusterName()
	keysDir := profile.FullProfilePath(tc.Config.KeysDir)
	ppkFilePath := keypaths.PPKFilePath(keysDir, proxyHost, tc.Config.Username)
	certificateFilePath := keypaths.SSHCertPath(keysDir, proxyHost, tc.Config.Username, rootClusterName)

	targetClusterName := rootClusterName
	if cf.LeafClusterName != "" {
		targetClusterName = cf.LeafClusterName
	}

	var hostCAPublicKeys map[string][]string
	hostCAPublicKeys, err = puttyhosts.ProcessHostCAPublicKeys(tc, cf.Context, targetClusterName)

	// update the cert path if a leaf cluster was requested
	proxyCommandClusterName := rootClusterName
	if cf.LeafClusterName != "" {
		// if we haven't found the requested leaf cluster, error out
		if _, ok := hostCAPublicKeys[cf.LeafClusterName]; !ok {
			return trace.NotFound("Cannot find registered leaf cluster %q. Use the leaf cluster name as it appears in the output of `tsh clusters`.", cf.LeafClusterName)
		}
		proxyCommandClusterName = cf.LeafClusterName
	}

	// format all the applicable host CAs into an intermediate data struct that can be added to the registry
	addToRegistry := puttyhosts.FormatHostCAPublicKeysForRegistry(hostCAPublicKeys, hostname)

	for cluster, values := range addToRegistry {
		for _, registryPublicKeyStruct := range values {
			if err := addHostCAPublicKey(registryPublicKeyStruct); err != nil {
				logger.ErrorContext(cf.Context, "Failed to add host CA key for cluster",
					"cluster", cluster,
					"error", err,
				)
				return trace.Wrap(err)
			}
			logger.DebugContext(cf.Context, "Added/updated host CA key for cluster", cluster, cluster)
		}
	}

	// format local command string (to run 'tsh proxy ssh')
	localCommandString, err := puttyhosts.FormatLocalCommandString(cf.executablePath, proxyCommandClusterName)
	if err != nil {
		return trace.Wrap(err)
	}

	// add session to registry
	if err := addPuTTYSession(proxyHost, hostname, port, login, ppkFilePath, certificateFilePath, localCommandString, cf.LeafClusterName); err != nil {
		logger.ErrorContext(cf.Context, "Failed to add PuTTY session",
			"user_host", userHostString,
			"error", err,
		)
		return trace.Wrap(err)
	}

	// handle leaf clusters
	if cf.LeafClusterName != "" {
		fmt.Printf("Added PuTTY session for %v [leaf:%v,proxy:%v]\n", userHostString, cf.LeafClusterName, proxyHost)
		return nil
	}

	fmt.Printf("Added PuTTY session for %v [proxy:%v]\n", userHostString, proxyHost)
	return nil
}
