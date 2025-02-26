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

package utils

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"math/rand/v2"
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
	"unicode"

	"github.com/gravitational/trace"
	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	apiutils "github.com/gravitational/teleport/api/utils"
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

// assert that CloseFunc implement io.Closer.
var _ io.Closer = (CloseFunc)(nil)

// CloseFunc is a helper used to implement io.Closer on a closure.
type CloseFunc func() error

func (cf CloseFunc) Close() error {
	return cf()
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
	slog.DebugContext(context.Background(), "Tracer started",
		"trace", t.Description)
	return t
}

// Stop logs stop of the trace
func (t *Tracer) Stop() *Tracer {
	slog.DebugContext(context.Background(), "Tracer completed",
		"trace", t.Description,
		"duration", time.Since(t.Started),
	)
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
	// If address is not an IP address, return it unchanged.
	if ip == nil && out.Host != "" {
		return out.String()
	}
	// if address is unspecified, e.g. all interfaces 0.0.0.0 or multicast,
	// replace with localhost that is clickable
	if len(ip) == 0 || ip.IsUnspecified() || ip.IsMulticast() {
		out.Host = fmt.Sprintf("127.0.0.1:%v", port)
	}
	return out.String()
}

// AsBool converts string to bool, in case of the value is empty
// or unknown, defaults to false
func AsBool(v string) bool {
	if v == "" {
		return false
	}
	out, _ := apiutils.ParseBool(v)
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

// IsValidHostname checks if a string represents a valid hostname.
func IsValidHostname(hostname string) bool {
	for _, label := range strings.Split(hostname, ".") {
		if len(validation.IsDNS1035Label(label)) > 0 {
			return false
		}
	}
	return true
}

// IsValidUnixUser checks if a string represents a valid
// UNIX username.
func IsValidUnixUser(u string) bool {
	// See http://www.unix.com/man-page/linux/8/useradd:
	//
	// On Debian, the only constraints are that usernames must neither start with a dash ('-')
	// nor contain a colon (':') or a whitespace (space: ' ', end of line: '\n', tabulation:
	// '\t', etc.). Note that using a slash ('/') may break the default algorithm for the
	// definition of the user's home directory.

	const maxUsernameLen = 32
	if len(u) > maxUsernameLen || len(u) == 0 || u[0] == '-' {
		return false
	}
	if strings.ContainsAny(u, ":/") {
		return false
	}
	for _, r := range u {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return false
		}
	}
	return true
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
		if errors.Is(err, fs.ErrPermission) {
			//do not convert to system error as this loses the ability to compare that it is a permission error
			return nil, err
		}
		return nil, trace.ConvertSystemError(err)
	}
	bytes, err := os.ReadFile(abs)
	if err != nil {
		if errors.Is(err, fs.ErrPermission) {
			//do not convert to system error as this loses the ability to compare that it is a permission error
			return nil, err
		}
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

// OpaqueAccessDenied returns a generic [trace.NotFoundError] if [err] is a [trace.NotFoundError] or
// a [trace.AccessDeniedError] so as to avoid leaking the existence of secret resources,
// for other error types it returns the original error.
func OpaqueAccessDenied(err error) error {
	if trace.IsNotFound(err) || trace.IsAccessDenied(err) {
		return trace.NotFound("not found")
	}
	return trace.Wrap(err)
}

// PortList is a list of TCP ports.
type PortList struct {
	ports []string
	sync.Mutex
}

// Pop returns a value from the list, it panics if the value is not there
func (p *PortList) Pop() string {
	p.Lock()
	defer p.Unlock()
	if len(p.ports) == 0 {
		panic("list is empty")
	}
	val := p.ports[len(p.ports)-1]
	p.ports = p.ports[:len(p.ports)-1]
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

// PortStartingNumber is a starting port number for tests
const PortStartingNumber = 20000

// GetFreeTCPPorts returns n ports starting from port 20000.
func GetFreeTCPPorts(n int, offset ...int) (PortList, error) {
	list := make([]string, 0, n)
	start := PortStartingNumber
	if len(offset) != 0 {
		start = offset[0]
	}
	for i := start; i < start+n; i++ {
		list = append(list, strconv.Itoa(i))
	}
	return PortList{ports: list}, nil
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

// ChooseRandomString returns a random string from the given slice.
func ChooseRandomString(slice []string) string {
	switch len(slice) {
	case 0:
		return ""
	case 1:
		return slice[0]
	default:
		return slice[rand.N(len(slice))]
	}
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
func AddrsFromStrings(s apiutils.Strings, defaultPort int) ([]NetAddr, error) {
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

// LimitReader returns a reader that limits bytes from r, and reports an error
// when limit bytes are read.
func LimitReader(r io.Reader, limit int64) io.Reader {
	return &limitedReader{
		LimitedReader: &io.LimitedReader{R: r, N: limit},
	}
}

// limitedReader wraps an [io.LimitedReader] that limits bytes read, and
// reports an error when the read limit is reached.
type limitedReader struct {
	*io.LimitedReader
}

func (l *limitedReader) Read(p []byte) (int, error) {
	n, err := l.LimitedReader.Read(p)
	if l.LimitedReader.N <= 0 {
		return n, ErrLimitReached
	}
	return n, err
}

// ReadAtMost reads up to limit bytes from r, and reports an error
// when limit bytes are read.
func ReadAtMost(r io.Reader, limit int64) ([]byte, error) {
	limitedReader := LimitReader(r, limit)
	data, err := io.ReadAll(limitedReader)
	return data, err
}

// HasPrefixAny determines if any of the string values have the given prefix.
func HasPrefixAny(prefix string, values []string) bool {
	for _, val := range values {
		if strings.HasPrefix(val, prefix) {
			return true
		}
	}

	return false
}

// ByteCount converts a size in bytes to a human-readable string.
func ByteCount(b int64) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB",
		float64(b)/float64(div), "kMGTPE"[exp])
}

// ErrLimitReached means that the read limit is reached.
//
// TODO(gavin): this should be converted to a 413 StatusRequestEntityTooLarge
// in trace.ErrorToCode instead of 429 StatusTooManyRequests.
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
	// CertTeleportClusterName is a name of the teleport cluster
	CertTeleportClusterName = "x-teleport-cluster-name"
	// CertTeleportUserCertificate is the certificate of the authenticated in user.
	CertTeleportUserCertificate = "x-teleport-certificate"
	// ExtIntCertType is an internal extension used to propagate cert type.
	ExtIntCertType = "certtype@teleport"
	// ExtIntCertTypeHost indicates a host-type certificate.
	ExtIntCertTypeHost = "host"
	// ExtIntCertTypeUser indicates a user-type certificate.
	ExtIntCertTypeUser = "user"
	// ExtIntSSHAccessPermit is an internal extension used to propagate
	// the access permit for the user.
	ExtIntSSHAccessPermit = "ssh-access-permit@teleport"
	// ExtIntSSHJoinPermi is an internal extension used to propagate
	// the join permit for the user.
	ExtIntSSHJoinPermit = "ssh-join-permit@teleport"
)
