/*
Copyright 2015 Gravitational, Inc.

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

package utils

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
)

// ParseBool parses string as boolean value,
// returns error in case if value is not recognized
func ParseBool(value string) (bool, error) {
	switch strings.ToLower(value) {
	case "yes", "yeah", "y", "true", "1", "on":
		return true, nil
	case "no", "nope", "n", "false", "0", "off":
		return false, nil
	default:
		return false, trace.BadParameter("unsupported value: %q", value)
	}
}

// ParseAdvertiseAddress validates advertise address,
// makes sure it's not an unreachable or multicast address
// returns address split into host and port, port could be empty
// if not specified
func ParseAdvertiseAddr(advertiseIP string) (string, string, error) {
	advertiseIP = strings.TrimSpace(advertiseIP)
	host := advertiseIP
	port := ""
	if len(net.ParseIP(host)) == 0 && strings.Contains(advertiseIP, ":") {
		var err error
		host, port, err = net.SplitHostPort(advertiseIP)
		if err != nil {
			return "", "", trace.BadParameter("failed to parse address %q", advertiseIP)
		}
		if _, err := strconv.Atoi(port); err != nil {
			return "", "", trace.BadParameter("bad port %q, expected integer", port)
		}
		if host == "" {
			return "", "", trace.BadParameter("missing host parameter")
		}
	}
	ip := net.ParseIP(host)
	if len(ip) != 0 {
		if ip.IsLoopback() || ip.IsUnspecified() || ip.IsMulticast() {
			return "", "", trace.BadParameter("unreachable advertise IP: %v", advertiseIP)
		}
	}
	return host, port, nil
}

// StringsSet creates set of string (map[string]struct{})
// from a list of strings
func StringsSet(in []string) map[string]struct{} {
	if in == nil {
		return nil
	}
	out := make(map[string]struct{})
	for _, v := range in {
		out[v] = struct{}{}
	}
	return out
}

// ParseOnOff parses whether value is "on" or "off", parameterName is passed for error
// reporting purposes, defaultValue is returned when no value is set
func ParseOnOff(parameterName, val string, defaultValue bool) (bool, error) {
	switch val {
	case teleport.On:
		return true, nil
	case teleport.Off:
		return false, nil
	case "":
		return defaultValue, nil
	default:
		return false, trace.BadParameter("bad %q parameter value: %q, supported values are on or off", parameterName, val)
	}
}

// IsGroupMember returns whether currently logged user is a member of a group
func IsGroupMember(gid int) (bool, error) {
	groups, err := os.Getgroups()
	if err != nil {
		return false, trace.ConvertSystemError(err)
	}
	for _, group := range groups {
		if group == gid {
			return true, nil
		}
	}
	return false, nil
}

// Host extracts host from host:port string
func Host(hostname string) (string, error) {
	if hostname == "" {
		return "", trace.BadParameter("missing parameter hostname")
	}
	if !strings.Contains(hostname, ":") {
		return hostname, nil
	}
	host, _, err := SplitHostPort(hostname)
	return host, err
}

// SplitHostPort splits host and port and checks that host is not empty
func SplitHostPort(hostname string) (string, string, error) {
	host, port, err := net.SplitHostPort(hostname)
	if err != nil {
		return "", "", trace.Wrap(err)
	}
	if host == "" {
		return "", "", trace.BadParameter("empty hostname")
	}
	return host, port, nil
}

func ReadPath(path string) ([]byte, error) {
	s, err := filepath.Abs(path)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	abs, err := filepath.EvalSymlinks(s)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	bytes, err := ioutil.ReadFile(abs)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	return bytes, nil
}

type multiCloser struct {
	closers []io.Closer
}

func (mc *multiCloser) Close() error {
	for _, closer := range mc.closers {
		if err := closer.Close(); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// MultiCloser implements io.Close, it sequentially calls Close() on each object
func MultiCloser(closers ...io.Closer) *multiCloser {
	return &multiCloser{
		closers: closers,
	}
}

// IsHandshakeFailedError specifies whether this error indicates
// failed handshake
func IsHandshakeFailedError(err error) bool {
	return strings.Contains(trace.Unwrap(err).Error(), "ssh: handshake failed")
}

// IsShellFailedError specifies whether this error indicates
// failed attempt to start shell
func IsShellFailedError(err error) bool {
	return strings.Contains(err.Error(), "ssh: cound not start shell")
}

// PortList is a list of TCP port
type PortList []string

// Pop returns a value from the list, it panics if the value is not there
func (p *PortList) Pop() string {
	if len(*p) == 0 {
		panic("list is empty")
	}
	val := (*p)[len(*p)-1]
	*p = (*p)[:len(*p)-1]
	return val
}

// GetFreeTCPPorts returns n ports starting from port 20000.
func GetFreeTCPPorts(n int) (PortList, error) {
	list := make(PortList, 0, n)
	for i := 20000; i < 20000+n; i++ {
		list = append(list, strconv.Itoa(i))
	}
	return list, nil
}

// ReadHostUUID reads host UUID from the file in the data dir
func ReadHostUUID(dataDir string) (string, error) {
	out, err := ReadPath(filepath.Join(dataDir, HostUUIDFile))
	if err != nil {
		return "", trace.Wrap(err)
	}
	return strings.TrimSpace(string(out)), nil
}

// WriteHostUUID writes host UUID into a file
func WriteHostUUID(dataDir string, id string) error {
	err := ioutil.WriteFile(filepath.Join(dataDir, HostUUIDFile), []byte(id), os.ModeExclusive|0400)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	return nil
}

// ReadOrMakeHostUUID looks for a hostid file in the data dir. If present,
// returns the UUID from it, otherwise generates one
func ReadOrMakeHostUUID(dataDir string) (string, error) {
	id, err := ReadHostUUID(dataDir)
	if err == nil {
		return id, nil
	}
	if !trace.IsNotFound(err) {
		return "", trace.Wrap(err)
	}
	id = uuid.New()
	if err = WriteHostUUID(dataDir, id); err != nil {
		return "", trace.Wrap(err)
	}
	return id, nil
}

// PrintVersion prints human readable version
func PrintVersion() {
	modules.GetModules().PrintVersion()
}

// HumanTimeFormat formats time as recognized by humans
func HumanTimeFormat(d time.Time) string {
	return d.Format(HumanTimeFormatString)
}

// Deduplicate deduplicates list of strings
func Deduplicate(in []string) []string {
	if len(in) == 0 {
		return in
	}
	out := make([]string, 0, len(in))
	seen := make(map[string]bool, len(in))
	for _, val := range in {
		if _, ok := seen[val]; !ok {
			out = append(out, val)
			seen[val] = true
		}
	}
	return out
}

// SliceContainsStr returns 'true' if the slice contains the given value
func SliceContainsStr(slice []string, value string) bool {
	for i := range slice {
		if slice[i] == value {
			return true
		}
	}
	return false
}

// RemoveFromSlice makes a copy of the slice and removes the passed in values from the copy.
func RemoveFromSlice(slice []string, values ...string) []string {
	output := make([]string, 0, len(slice))

	remove := make(map[string]bool)
	for _, value := range values {
		remove[value] = true
	}

	for _, s := range slice {
		_, ok := remove[s]
		if ok {
			continue
		}
		output = append(output, s)
	}

	return output
}

// CheckCertificateFormatFlag checks if the certificate format is valid.
func CheckCertificateFormatFlag(s string) (string, error) {
	switch s {
	case teleport.CertificateFormatStandard, teleport.CertificateFormatOldSSH, teleport.CertificateFormatUnspecified:
		return s, nil
	default:
		return "", trace.BadParameter("invalid certificate format parameter: %q", s)
	}
}

// Strings is a list of string that can unmarshal from list of strings
// or a scalar string from scalar yaml or json property
type Strings []string

// UnmarshalJSON unmarshals scalar string or strings slice to Strings
func (s *Strings) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	var stringVar string
	if err := json.Unmarshal(data, &stringVar); err == nil {
		*s = []string{stringVar}
		return nil
	}
	var stringsVar []string
	if err := json.Unmarshal(data, &stringsVar); err != nil {
		return trace.Wrap(err)
	}
	*s = stringsVar
	return nil
}

// UnmarshalYAML is used to allow Strings to unmarshal from
// scalar string value or from the list
func (s *Strings) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// try unmarshal as string
	var val string
	err := unmarshal(&val)
	if err == nil {
		*s = []string{val}
		return nil
	}

	// try unmarshal as slice
	var slice []string
	err = unmarshal(&slice)
	if err == nil {
		*s = slice
		return nil
	}

	return err
}

// MarshalJSON marshals to scalar value
// if there is only one value in the list
// to list otherwise
func (s Strings) MarshalJSON() ([]byte, error) {
	if len(s) == 1 {
		return json.Marshal(s[0])
	}
	return json.Marshal([]string(s))
}

// MarshalYAML marshals to scalar value
// if there is only one value in the list,
// marshals to list otherwise
func (s Strings) MarshalYAML() (interface{}, error) {
	if len(s) == 1 {
		return s[0], nil
	}
	return []string(s), nil
}

// Addrs returns strings list converted to address list
func (s Strings) Addrs(defaultPort int) ([]NetAddr, error) {
	addrs := make([]NetAddr, len(s))
	for i, val := range s {
		addr, err := ParseHostPortAddr(val, defaultPort)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		addrs[i] = *addr
	}
	return addrs, nil
}

const (
	// HumanTimeFormatString is a human readable date formatting
	HumanTimeFormatString = "Mon Jan _2 15:04 UTC"
	// CertTeleportUser specifies teleport user
	CertTeleportUser = "x-teleport-user"
	// CertTeleportUserCA specifies teleport certificate authority
	CertTeleportUserCA = "x-teleport-user-ca"
	// CertExtensionRole specifies teleport role
	CertExtensionRole = "x-teleport-role"
	// CertExtensionAuthority specifies teleport authority's name
	// that signed this domain
	CertExtensionAuthority = "x-teleport-authority"
	// HostUUIDFile is the file name where the host UUID file is stored
	HostUUIDFile = "host_uuid"
	// CertTeleportClusterName is a name of the teleport cluster
	CertTeleportClusterName = "x-teleport-cluster-name"
	// CertTeleportUserCertificate is the certificate of the authenticated in user.
	CertTeleportUserCertificate = "x-teleport-certificate"
)
