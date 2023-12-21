//go:build bpf && !386
// +build bpf,!386

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

package restrictedsession

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"
	"unsafe"

	"github.com/aquasecurity/libbpfgo"
	"github.com/gravitational/trace"
)

var (
	wildcard4 = net.IPNet{
		IP:   net.IP{0, 0, 0, 0},
		Mask: net.IPMask{0, 0, 0, 0},
	}

	wildcard6 = net.IPNet{
		IP:   net.IP{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		Mask: net.IPMask{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	}
)

// ipTrie wraps BPF LSM map to work with net.IPNet types
type ipTrie struct {
	bpfMap *libbpfgo.BPFMap
}

func newIPTrie(m *libbpfgo.Module, name string) (ipTrie, error) {
	t, err := m.GetMap(name)
	if err != nil {
		return ipTrie{}, trace.Wrap(err)
	}

	return ipTrie{
		bpfMap: t,
	}, nil
}

func (t *ipTrie) toKey(n net.IPNet) []byte {
	prefixLen, _ := n.Mask.Size()

	// Key format: Prefix length (4 bytes) followed by prefix
	key := make([]byte, 4+len(n.IP))

	binary.LittleEndian.PutUint32(key[0:4], uint32(prefixLen))
	copy(key[4:], n.IP)

	return key
}

// Add upserts (prefixLen, prefix) -> value entry in BPF trie
func (t *ipTrie) add(n net.IPNet) error {
	key := t.toKey(n)
	return t.bpfMap.Update(unsafe.Pointer(&key[0]), unsafe.Pointer(&unit[0]))
}

// Remove removes the entry for the given network
func (t *ipTrie) remove(n net.IPNet) error {
	key := t.toKey(n)
	return t.bpfMap.DeleteKey(unsafe.Pointer(&key[0]))
}

// cmp is 3-way integral compare
func cmp(x, y int) int {
	switch {
	case x < y:
		return -1
	case x > y:
		return 1
	default:
		return 0
	}
}

// prefixLen returns the length of network prefix
// based on IPv6 encoding
func prefixLen(m net.IPMask) int {
	ones, bits := m.Size()
	if bits == 32 {
		ones += (128 - 32)
	}
	return ones
}

// compareIPNets performs a 3-way compare of two IPNet
// objects. This induces a total order but it doesn't
// matter what order that is.
func compareIPNets(x, y *net.IPNet) int {
	x.IP = x.IP.To16()
	y.IP = y.IP.To16()

	if ret := bytes.Compare(x.IP, y.IP); ret != 0 {
		return ret
	}

	xPrefix := prefixLen(x.Mask)
	yPrefix := prefixLen(y.Mask)

	return cmp(xPrefix, yPrefix)
}

// ipNets is sort.Interface impl to sort []net.IPNet
type ipNets []net.IPNet

func (s ipNets) Len() int {
	return len(s)
}

func (s ipNets) Less(i, j int) bool {
	x := &s[i]
	y := &s[j]
	return compareIPNets(x, y) < 0
}

func (s ipNets) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// diffIPNets computes the differences between prev
// and next sets of net.IPNet objects. The diffs are
// returned as a tuple of (additions, deletions)
func diffIPNets(prev, next ipNets) (ipNets, ipNets) {
	// Sorts []net.IPNet according to some
	// criteria. The ordering used is immaterial, as long
	// as total order is induced.
	sort.Sort(prev)
	sort.Sort(next)

	adds := ipNets{}
	dels := ipNets{}

	i := 0
	j := 0
	for i < len(prev) && j < len(next) {
		switch compareIPNets(&prev[i], &next[j]) {
		case -1:
			dels = append(dels, prev[i])
			i++
		case 0:
			i++
			j++
		case 1:
			adds = append(adds, next[j])
			j++
		}
	}

	// handle the tails (at most one of the lists still has a tail)
	dels = append(dels, prev[i:]...)
	adds = append(adds, next[j:]...)

	return adds, dels
}

// network restricts IPv4 and IPv6 related operations.
type network struct {
	mu           sync.Mutex
	mod          *libbpfgo.Module
	deny4        ipTrie
	allow4       ipTrie
	deny6        ipTrie
	allow6       ipTrie
	restrictions *NetworkRestrictions
}

func newNetwork(mod *libbpfgo.Module) (*network, error) {
	deny4, err := newIPTrie(mod, "ip4_denylist")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	allow4, err := newIPTrie(mod, "ip4_allowlist")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	deny6, err := newIPTrie(mod, "ip6_denylist")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	allow6, err := newIPTrie(mod, "ip6_allowlist")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	n := network{
		mod:          mod,
		deny4:        deny4,
		allow4:       allow4,
		deny6:        deny6,
		allow6:       allow6,
		restrictions: &NetworkRestrictions{},
	}

	if err = n.start(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &n, err
}

func (n *network) start() error {
	hooks := []string{"socket_connect", "socket_sendmsg"}

	for _, hook := range hooks {
		if err := attachLSM(n.mod, hook); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

func ipv4MappedIPNet(ipnet net.IPNet) net.IPNet {
	ipnet.IP = ipnet.IP.To16()
	ones, _ := ipnet.Mask.Size()
	// IPv4 mapped address has a 96-bit fixed prefix
	ipnet.Mask = net.CIDRMask(96+ones, 128)
	return ipnet
}

func (n *network) apply(nets []net.IPNet, fn4, fn6 func(net.IPNet) error) error {
	for _, ipnet := range nets {
		ip := ipnet.IP.To4()
		if ip != nil {
			// IPv4 address
			ipnet.IP = ip
			if err := fn4(ipnet); err != nil {
				return trace.Wrap(err)
			}

			// Also add it to IPv6 trie as a mapped address.
			// Needed in case an AF_INET6 socket is used with
			// IPv4 translated address. The IPv6 stack will forward
			// it to IPv4 stack but that happens much lower than
			// the LSM hook.
			ipnet = ipv4MappedIPNet(ipnet)
			if err := fn6(ipnet); err != nil {
				return trace.Wrap(err)
			}
		} else {
			ip = ipnet.IP.To16()
			if ip == nil {
				return fmt.Errorf("%q is not an IPv4 or IPv6 address", ip.String())
			}

			if err := fn6(ipnet); err != nil {
				return trace.Wrap(err)
			}
		}
	}

	return nil
}

func (n *network) update(newRestrictions *NetworkRestrictions) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if !newRestrictions.Enabled {
		newRestrictions.Allow = []net.IPNet{wildcard4, wildcard6}
		newRestrictions.Deny = nil
	}

	// Compute the diff between the previous and new configs
	// as a set of additions and deletions. Then apply these
	// changes to the BPF maps.

	// The deny list
	denyAdds, denyDels := diffIPNets(n.restrictions.Deny, newRestrictions.Deny)
	// Do the deletions
	if err := n.apply(denyDels, n.deny4.remove, n.deny6.remove); err != nil {
		return trace.Wrap(err)
	}
	// Do the additions
	if err := n.apply(denyAdds, n.deny4.add, n.deny6.add); err != nil {
		return trace.Wrap(err)
	}

	// The allow list
	allowAdds, allowDels := diffIPNets(n.restrictions.Allow, newRestrictions.Allow)
	// Do the deletions
	if err := n.apply(allowDels, n.allow4.remove, n.allow6.remove); err != nil {
		return trace.Wrap(err)
	}
	// Do the additions
	if err := n.apply(allowAdds, n.allow4.add, n.allow6.add); err != nil {
		return trace.Wrap(err)
	}

	n.restrictions = newRestrictions

	log.Infof("New network restrictions applied: allow=[%v], deny=[%v]",
		ipNetsToString(n.restrictions.Allow),
		ipNetsToString(n.restrictions.Deny))

	return nil
}

func (n *network) close() {
}

func ipNetsToString(ns []net.IPNet) string {
	b := strings.Builder{}
	for i, n := range ns {
		if i != 0 {
			b.WriteString(", ")
		}

		b.WriteString(n.String())
	}

	return b.String()
}
