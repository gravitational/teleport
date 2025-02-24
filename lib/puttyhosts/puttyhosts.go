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

package puttyhosts

import (
	"context"
	"encoding/base64"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"text/template"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/sshutils"
)

type PuttyProxyTelnetCommandArgs struct {
	TSHPath string
	Cluster string
}

type HostCAPublicKeyForRegistry struct {
	KeyName   string
	PublicKey string
	Hostname  string
}

func hostnameContainsDot(hostname string) bool {
	return strings.Contains(hostname, ".")
}

func hostnameIsWildcard(hostname string) bool {
	return strings.HasPrefix(hostname, "*.")
}

func wildcardFromHostname(hostname string) string {
	if hostnameIsWildcard(hostname) {
		return hostname
	}
	// prevent a panic below if the string doesn't contain a hostname. this should never happen,
	// as this function is only intended to be called after checking hostnameContainsDot.
	if !hostnameContainsDot(hostname) {
		return hostname
	}
	return fmt.Sprintf("*.%s", strings.Join(strings.Split(hostname, ".")[1:], "."))
}

// AddHostToHostList adds a new hostname to PuTTY's list of trusted hostnames for a given host CA.
//
// Background:
//   - For every host CA that it is configured to trust, PuTTY maintains a list of hostnames (hostList) which it should consider
//     to be valid for that host CA. This is the same as the @cert-authority lines in an `~/.ssh/known_hosts` file.
//   - Trusted hostnames can be individual entries (host1, host2) or wildcards like "*.example.com".
//   - PuTTY keeps this list of hostnames stored against each host CA in the Windows registry. It exposes a GUI (under
//     Connection -> SSH -> Host Keys -> Configure Host CAs at the time of writing) which expects any new host CAs and
//     trusted hostnames for each to be added manually by end users as part of session configuration.
//   - This process is mandatory for validation of host CAs in PuTTY to work, but is a cumbersome manual process with many
//     clicks required in a nested interface. Instead, this function is called as part of `tsh puttyconfig` to examine the
//     existing list of trusted hostnames and automate the process of adding a new valid hostname to a given host CA.
//
// Connection flow:
//   - When connecting to a host which presents a host CA, PuTTY searches its list of CAs to find any which are considered
//     valid for that hostname, then checks whether the host's presented CA matches any of them. If there is a CA -> hostname
//     match, the connection will continue successfully. If not, an error will be shown.
//
// Intended operation of this function:
//   - This function is passed the current list of trusted hostnames for a given host CA (retrieved from the registry), along
//     with a new hostname entry (from tsh puttyconfig <hostname>) which should be added to the list.
//   - It appends the new hostname to the end of the hostList
//   - All hostnames in the hostList are converted to their wildcard form if they contain a dot (test.example.com -> *.example.com)
//     and are grouped together.
//   - If a wildcard group only contains a single hostname which would be matched by its wildcard equivalent, that hostname is added
//     to the hostList verbatim to prevent inadvertently matching against too many hosts with the same wildcard.
//   - If a wildcard matches more than one hostname, the wildcard will be added to the hostList instead and the single hostnames
//     discarded.
//   - The hostList is then sorted alphabetically and returned.
//
// This is an effort to keep the length of hostList as short as possible for efficiency and tidiness, while not using any more
// wildcards than necessary and preventing the need for end users to manually configure their trusted host CAs.
func AddHostToHostList(hostList []string, hostname string) []string {
	// add the incoming hostname to the hostList before we sort and process it
	hostList = append(hostList, hostname)

	hostMap := make(map[string][]string)
	var extraHosts []string
	// iterate over the full hostList
	// if the element is a wildcard, add it to the list of wildcards
	// if the element is not a wildcard, convert it to a wildcard and add any hostnames it matches to a map
	for _, element := range hostList {
		// FQDN-based hosts are grouped under a wildcard key
		if hostnameContainsDot(element) {
			wildcard := wildcardFromHostname(element)
			if !slices.Contains(hostMap[wildcard], element) {
				hostMap[wildcard] = append(hostMap[wildcard], element)
			}
		} else {
			// any non-wildcard hosts go into the extraHosts list and will be processed separately
			extraHosts = append(extraHosts, element)
		}
	}

	var outputHostList []string
	// first, add all non-wildcard matches separately
	for _, hostname := range extraHosts {
		if !slices.Contains(outputHostList, hostname) {
			outputHostList = append(outputHostList, hostname)
		}
	}
	// iterate over the map, look for all wildcard keys with more than one hostname matching.
	// for each match, add the wildcard to the hostList.
	for key, matchingHostnames := range hostMap {
		// add all wildcards with more than one hostname matching
		if len(matchingHostnames) > 1 {
			outputHostList = append(outputHostList, key)
		} else {
			// add the single hostname which is matched by the given wildcard
			outputHostList = append(outputHostList, matchingHostnames[0])
		}
	}

	slices.Sort(outputHostList)
	return outputHostList
}

var hostnameRegexp = regexp.MustCompile(`^([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9-]*[a-zA-Z0-9])(\.([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9-]*[a-zA-Z0-9]))*$`)

// NaivelyValidateHostname checks the provided hostname against a naive regex to ensure it doesn't contain obviously
// illegal characters. It's not guaranteed to be perfect, just a simple sanity check. It returns true when the hostname validates.
func NaivelyValidateHostname(hostname string) bool {
	return hostnameRegexp.MatchString(hostname)
}

// FormatLocalCommandString replaces placeholders in a constant with actual values
func FormatLocalCommandString(tshPath string, cluster string) (string, error) {
	// PuTTY needs its paths to be double-escaped i.e. C:\\Users\\User\\tsh.exe
	escapedTSHPath := strings.ReplaceAll(tshPath, `\`, `\\`)
	// build the command using a template
	templateString := "{{.TSHPath}} proxy ssh --cluster={{.Cluster}} --proxy=%proxyhost %user@%host:%port"
	localCommandTemplate := template.Must(template.New("puttyProxyTelnetCommand").Parse(templateString))
	var builder strings.Builder
	err := localCommandTemplate.Execute(&builder, PuttyProxyTelnetCommandArgs{
		TSHPath: escapedTSHPath,
		Cluster: cluster,
	})
	if err != nil {
		return "", trace.Wrap(err)
	}
	return builder.String(), nil
}

// getAllHostCAs queries the root cluster for its host CAs
func getAllHostCAs(tc *client.TeleportClient, cfContext context.Context) ([]types.CertAuthority, error) {
	var err error
	// get all CAs for the cluster (including trusted clusters)
	var cas []types.CertAuthority
	err = tc.WithRootClusterClient(cfContext, func(clt authclient.ClientI) error {
		cas, err = clt.GetCertAuthorities(cfContext, types.HostCA, false /* exportSecrets */)
		if err != nil {
			return trace.Wrap(err)
		}
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return cas, nil
}

// ProcessHostCAPublicKeys gets all the host CAs that the passed client can load (which will be a root cluster and any connected leaf clusters),
// iterates over them to find any host CAs which map to the requested root or leaf cluster and builds a map containing [targetClusterName]->[]CAs.
// These host CA public keys are then ultimately written to the registry so that PuTTY can validate host keys against them when connecting.
func ProcessHostCAPublicKeys(tc *client.TeleportClient, cfContext context.Context, clusterName string) (map[string][]string, error) {
	// iterate over all the CAs
	hostCAPublicKeys := make(map[string][]string)
	hostCAs, err := getAllHostCAs(tc, cfContext)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, ca := range hostCAs {
		// if this is either the root or the requested leaf cluster, process it
		if ca.GetName() == clusterName {
			for _, key := range ca.GetTrustedSSHKeyPairs() {
				kh, err := sshutils.MarshalKnownHost(sshutils.KnownHost{
					Hostname:      ca.GetClusterName(),
					AuthorizedKey: key.PublicKey,
				})
				if err != nil {
					return nil, trace.Wrap(err)
				}
				_, _, hostCABytes, _, _, err := ssh.ParseKnownHosts([]byte(kh))
				if err != nil {
					return nil, trace.Wrap(err)
				}

				hostCAPublicKey := base64.StdEncoding.EncodeToString(hostCABytes.Marshal())
				hostCAPublicKeys[ca.GetName()] = append(hostCAPublicKeys[ca.GetName()], hostCAPublicKey)
			}
		}
	}
	return hostCAPublicKeys, nil
}

// FormatHostCAPublicKeysFoRegistry formats a map of clusterNames -> []CAs into a platform-agnostic intermediate
// struct format. This format is passed into functions which write to the Windows registry.
func FormatHostCAPublicKeysForRegistry(hostCAPublicKeys map[string][]string, hostname string) map[string][]HostCAPublicKeyForRegistry {
	registryOutput := make(map[string][]HostCAPublicKeyForRegistry)
	// add all host CA public keys for cluster
	for cluster, hostCAs := range hostCAPublicKeys {
		baseKeyName := fmt.Sprintf(`TeleportHostCA-%v`, cluster)
		for i, publicKey := range hostCAs {
			// append indices to entries if we have multiple public keys for a CA
			keyName := baseKeyName
			if len(hostCAs) > 1 {
				keyName = fmt.Sprintf(`%v-%d`, baseKeyName, i)
			}
			registryOutput[cluster] = append(registryOutput[cluster], HostCAPublicKeyForRegistry{keyName, publicKey, hostname})
		}
	}
	return registryOutput
}

// CheckAndSplitValidityKey processes PuTTY's "Validity" string key into individual list elements
// and checks that its formatting follows the simple pattern "<hostname> || <hostname> || ..."
// PuTTY uses a custom string format to represent what hostnames a given key should be trusted for.
// See https://the.earth.li/~sgtatham/putty/0.79/htmldoc/Chapter4.html#config-ssh-cert-valid-expr for details.
func CheckAndSplitValidityKey(input string, caName string) ([]string, error) {
	var output []string
	docsURL := "https://goteleport.com/docs/connect-your-client/putty-winscp/#troubleshooting"

	// if the input string has no content (because the Validity key has no value yet), return the empty list
	if len(input) == 0 {
		return output, nil
	}

	// split the input string on spaces
	splitTokens := strings.Fields(input)

	// if the total number of hostnames and tokens doesn't equal an odd number, we can return an error early as the string is invalid
	if len(splitTokens)%2 != 1 {
		return nil, trace.BadParameter("validity string for %v contains an even [%d] number of entries but should be odd, see %v",
			caName, len(splitTokens), docsURL)
	}
	for index, token := range splitTokens {
		// check that every odd value in the string (zero-indexed) is equal to the OR splitter "||" and return an error if not
		if index%2 == 1 {
			if token != "||" {
				return nil, trace.BadParameter("validity string for %v contains invalid splitter token %q in field %d, see %v",
					caName, token, index, docsURL)
			}
		} else {
			// if the || delimiter is in a non-odd position, return an error
			if token == "||" {
				return nil, trace.BadParameter("validity string for %v contains consecutive splitter tokens with no hostname in field %d, see %v",
					caName, index, docsURL)
			}
			// if the string contains any value which is not part of a hostname, return an error
			if badIndex := strings.IndexAny(token, "()&!:|"); badIndex != -1 {
				return nil, trace.BadParameter("validity string for %v contains an invalid character %q in field %v, see %v",
					caName, token[badIndex], index, docsURL)
			}
			// check the token using the naive hostname regex and return an error if it doesn't match
			if !hostnameIsWildcard(token) && !NaivelyValidateHostname(token) {
				return nil, trace.BadParameter("validity string for %v appears to contain non-hostname %q in field %v, see %v",
					caName, token, index, docsURL)
			}
			output = append(output, token)
		}
	}

	return output, nil
}
