/*
Copyright 2015-2020 Gravitational, Inc.

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
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
	log "github.com/sirupsen/logrus"
)

// WriteContextCloser provides close method with context
type WriteContextCloser interface {
	Close(ctx context.Context) error
	io.Writer
}

// WriteCloserWithContext converts ContextCloser to io.Closer,
// whenever new Close method will be called, the ctx will be passed to it
func WriteCloserWithContext(ctx context.Context, closer WriteContextCloser) io.WriteCloser {
	return &closerWithContext{
		WriteContextCloser: closer,
		ctx:                ctx,
	}
}

type closerWithContext struct {
	WriteContextCloser
	ctx context.Context
}

// Close closes all resources and returns the result
func (c *closerWithContext) Close() error {
	return c.WriteContextCloser.Close(c.ctx)
}

// NilCloser returns closer if it's not nil
// otherwise returns a nop closer
func NilCloser(r io.Closer) io.Closer {
	if r == nil {
		return &nilCloser{}
	}
	return r
}

type nilCloser struct {
}

func (*nilCloser) Close() error {
	return nil
}

// NopWriteCloser returns a WriteCloser with a no-op Close method wrapping
// the provided Writer w
func NopWriteCloser(r io.Writer) io.WriteCloser {
	return nopWriteCloser{r}
}

type nopWriteCloser struct {
	io.Writer
}

func (nopWriteCloser) Close() error { return nil }

// Tracer helps to trace execution of functions
type Tracer struct {
	// Started records starting time of the call
	Started time.Time
	// Description is arbitrary description
	Description string
}

// NewTracer returns a new tracer
func NewTracer(description string) *Tracer {
	return &Tracer{Started: time.Now().UTC(), Description: description}
}

// Start logs start of the trace
func (t *Tracer) Start() *Tracer {
	log.Debugf("Tracer started %v.", t.Description)
	return t
}

// Stop logs stop of the trace
func (t *Tracer) Stop() *Tracer {
	log.Debugf("Tracer completed %v in %v.", t.Description, time.Since(t.Started))
	return t
}

// ThisFunction returns calling function name
func ThisFunction() string {
	var pc [32]uintptr
	runtime.Callers(2, pc[:])
	return runtime.FuncForPC(pc[0]).Name()
}

// SyncString is a string value
// that can be concurrently accessed
type SyncString struct {
	sync.Mutex
	string
}

// Value returns value of the string
func (s *SyncString) Value() string {
	s.Lock()
	defer s.Unlock()
	return s.string
}

// Set sets the value of the string
func (s *SyncString) Set(v string) {
	s.Lock()
	defer s.Unlock()
	s.string = v
}

// ClickableURL fixes address in url to make sure
// it's clickable, e.g. it replaces "undefined" address like
// 0.0.0.0 used in network listeners format with loopback 127.0.0.1
func ClickableURL(in string) string {
	out, err := url.Parse(in)
	if err != nil {
		return in
	}
	host, port, err := net.SplitHostPort(out.Host)
	if err != nil {
		return in
	}
	ip := net.ParseIP(host)
	// if address is not an IP, unspecified, e.g. all interfaces 0.0.0.0 or multicast,
	// replace with localhost that is clickable
	if len(ip) == 0 || ip.IsUnspecified() || ip.IsMulticast() {
		out.Host = fmt.Sprintf("127.0.0.1:%v", port)
		return out.String()
	}
	return out.String()
}

// AsBool converts string to bool, in case of the value is empty
// or unknown, defaults to false
func AsBool(v string) bool {
	if v == "" {
		return false
	}
	out, _ := ParseBool(v)
	return out
}

// ParseAdvertiseAddr validates advertise address,
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
		if ip.IsUnspecified() || ip.IsMulticast() {
			return "", "", trace.BadParameter("unreachable advertise IP: %v", advertiseIP)
		}
	}
	return host, port, nil
}

// StringsSliceFromSet returns a sorted strings slice from set
func StringsSliceFromSet(in map[string]struct{}) []string {
	if in == nil {
		return nil
	}
	out := make([]string, 0, len(in))
	for key := range in {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

// StringsSet creates set of string (map[string]struct{})
// from a list of strings
func StringsSet(in []string) map[string]struct{} {
	if in == nil {
		return map[string]struct{}{}
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

// DNSName extracts DNS name from host:port string.
func DNSName(hostport string) (string, error) {
	host, err := Host(hostport)
	if err != nil {
		return "", trace.Wrap(err)
	}
	if ip := net.ParseIP(host); len(ip) != 0 {
		return "", trace.BadParameter("%v is an IP address", host)
	}
	return host, nil
}

// Host extracts host from host:port string
func Host(hostname string) (string, error) {
	if hostname == "" {
		return "", trace.BadParameter("missing parameter hostname")
	}
	// if this is IPv4 or V6, return as is
	if ip := net.ParseIP(hostname); len(ip) != 0 {
		return hostname, nil
	}
	// has no indication of port, return, note that
	// it will not break ipv6 as it always has at least one colon
	if !strings.Contains(hostname, ":") {
		return hostname, nil
	}
	host, _, err := SplitHostPort(hostname)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return host, nil
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

// ReadPath reads file contents
func ReadPath(path string) ([]byte, error) {
	if path == "" {
		return nil, trace.NotFound("empty path")
	}
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
func MultiCloser(closers ...io.Closer) io.Closer {
	return &multiCloser{
		closers: closers,
	}
}

// IsHandshakeFailedError specifies whether this error indicates
// failed handshake
func IsHandshakeFailedError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(trace.Unwrap(err).Error(), "ssh: handshake failed")
}

// IsCertExpiredError specifies whether this error indicates
// expired SSH certificate
func IsCertExpiredError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(trace.Unwrap(err).Error(), "ssh: cert has expired")
}

// OpaqueAccessDenied returns a generic NotFound instead of AccessDenied
// so as to avoid leaking the existence of secret resources.
func OpaqueAccessDenied(err error) error {
	if trace.IsAccessDenied(err) {
		return trace.NotFound("not found")
	}
	return trace.Wrap(err)
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

// PopInt returns a value from the list, it panics if not enough values
// were allocated
func (p *PortList) PopInt() int {
	i, err := strconv.Atoi(p.Pop())
	if err != nil {
		panic(err)
	}
	return i
}

// PopIntSlice returns a slice of values from the list, it panics if not enough
// ports were allocated
func (p *PortList) PopIntSlice(num int) []int {
	ports := make([]int, num)
	for i := range ports {
		ports[i] = p.PopInt()
	}
	return ports
}

// PortStartingNumber is a starting port number for tests
const PortStartingNumber = 20000

// GetFreeTCPPorts returns n ports starting from port 20000.
func GetFreeTCPPorts(n int, offset ...int) (PortList, error) {
	list := make(PortList, 0, n)
	start := PortStartingNumber
	if len(offset) != 0 {
		start = offset[0]
	}
	for i := start; i < start+n; i++ {
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

// StringSliceSubset returns true if b is a subset of a.
func StringSliceSubset(a []string, b []string) error {
	aset := make(map[string]bool)
	for _, v := range a {
		aset[v] = true
	}

	for _, v := range b {
		_, ok := aset[v]
		if !ok {
			return trace.BadParameter("%v not in set", v)
		}

	}
	return nil
}

// UintSliceSubset returns true if b is a subset of a.
func UintSliceSubset(a []uint16, b []uint16) error {
	aset := make(map[uint16]bool)
	for _, v := range a {
		aset[v] = true
	}

	for _, v := range b {
		_, ok := aset[v]
		if !ok {
			return trace.BadParameter("%v not in set", v)
		}

	}
	return nil
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
	case constants.CertificateFormatStandard, teleport.CertificateFormatOldSSH, teleport.CertificateFormatUnspecified:
		return s, nil
	default:
		return "", trace.BadParameter("invalid certificate format parameter: %q", s)
	}
}

// AddrsFromStrings returns strings list converted to address list
func AddrsFromStrings(s Strings, defaultPort int) ([]NetAddr, error) {
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

// FileExists checks whether a file exists at a given path
func FileExists(fp string) bool {
	_, err := os.Stat(fp)
	if err != nil && os.IsNotExist(err) {
		return false
	}
	return true
}

// StoreErrorOf stores the error returned by f within *err.
func StoreErrorOf(f func() error, err *error) {
	*err = trace.NewAggregate(*err, f())
}

// ReadAtMost reads up to limit bytes from r, and reports an error
// when limit bytes are read.
func ReadAtMost(r io.Reader, limit int64) ([]byte, error) {
	limitedReader := &io.LimitedReader{R: r, N: limit}
	data, err := ioutil.ReadAll(limitedReader)
	if err != nil {
		return data, err
	}
	if limitedReader.N <= 0 {
		return data, ErrLimitReached
	}
	return data, nil
}

// ErrLimitReached means that the read limit is reached.
var ErrLimitReached = &trace.LimitExceededError{Message: "the read limit is reached"}

const (
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
