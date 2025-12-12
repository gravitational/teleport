// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package vnet

import (
	"context"
	"errors"
	"net"
	"slices"
	"strconv"
	"strings"

	"github.com/gravitational/trace"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

var (
	userenv             = windows.NewLazySystemDLL("userenv.dll")
	procRefreshPolicyEx = userenv.NewProc("RefreshPolicyEx")
)

// platformOSConfigState holds state about which addresses and routes have
// already been configured in the OS. Experimentally, IP routing seems to be
// flaky/broken on Windows when the same routes are repeatedly configured, as we
// currently do on MacOS. Avoid this by only configuring each IP or route once.
//
// TODO(nklaassen): it would probably be better to read the current routing
// table from the OS, compute a diff, and reconcile the routes that we need.
// This works for now but if something else overwrites our deletes our routes,
// we'll never reset them.
type platformOSConfigState struct {
	configuredV4Address bool
	configuredV6Address bool
	configuredRanges    []string

	ifaceIndex string

	// configuredDNSZones caches DNS zones so DNS is reconfigured when they change.
	configuredDNSZones []string
	// configuredDNSAddrs caches DNS addresses so DNS is reconfigured when they change.
	configuredDNSAddrs []string
	// configuredGroupPolicyKey caches existence of the group policy key so DNS is reconfigured when
	// the key is created or removed.
	configuredGroupPolicyKey bool
}

func (p *platformOSConfigState) getIfaceIndex() (string, error) {
	if p.ifaceIndex != "" {
		return p.ifaceIndex, nil
	}
	iface, err := net.InterfaceByName(tunInterfaceName)
	if err != nil {
		return "", trace.Wrap(err, "looking up TUN interface by name %s", tunInterfaceName)
	}
	p.ifaceIndex = strconv.Itoa(iface.Index)
	return p.ifaceIndex, nil
}

// platformConfigureOS configures the host OS according to cfg. It is safe to
// call repeatedly, and it is meant to be called with an empty osConfig to
// deconfigure anything necessary before exiting.
func platformConfigureOS(ctx context.Context, cfg *osConfig, state *platformOSConfigState) error {
	// There is no need to remove IP addresses or routes, they will automatically be cleaned up when the
	// process exits and the TUN is deleted.

	if cfg.tunIPv4 != "" {
		if !state.configuredV4Address {
			log.InfoContext(ctx, "Setting IPv4 address for the TUN device",
				"device", cfg.tunName, "address", cfg.tunIPv4)
			netMask := maskForIPNet(cfg.tunIPv4Net)
			if err := runCommand(ctx,
				"netsh", "interface", "ip", "set", "address", cfg.tunName, "static", cfg.tunIPv4, netMask,
			); err != nil {
				return trace.Wrap(err)
			}
			state.configuredV4Address = true
		}
		for _, cidrRange := range cfg.cidrRanges {
			if slices.Contains(state.configuredRanges, cidrRange) {
				continue
			}
			log.InfoContext(ctx, "Routing CIDR range to the TUN IP",
				"device", cfg.tunName, "range", cidrRange)
			ifaceIndex, err := state.getIfaceIndex()
			if err != nil {
				return trace.Wrap(err, "getting index for TUN interface")
			}
			addr, mask, err := addrMaskForCIDR(cidrRange)
			if err != nil {
				return trace.Wrap(err)
			}
			if err := runCommand(ctx,
				"route", "add", addr, "mask", mask, cfg.tunIPv4, "if", ifaceIndex,
			); err != nil {
				return trace.Wrap(err)
			}
			state.configuredRanges = append(state.configuredRanges, cidrRange)
		}
	}

	if cfg.tunIPv6 != "" && !state.configuredV6Address {
		// It looks like we don't need to explicitly set a route for the IPv6
		// ULA prefix, assigning the address to the interface is enough.
		log.InfoContext(ctx, "Setting IPv6 address for the TUN device.",
			"device", cfg.tunName, "address", cfg.tunIPv6)
		if err := runCommand(ctx,
			"netsh", "interface", "ipv6", "set", "address", cfg.tunName, cfg.tunIPv6,
		); err != nil {
			return trace.Wrap(err)
		}
		state.configuredV6Address = true
	}

	// Configure DNS only if the DNS zones or addresses have changed. This typically happens when the
	// user logs in or out of a cluster. Otherwise configureDNS would refresh all computer policies
	// every 10 seconds when platformConfigureOS is called.
	doesGroupPolicyKeyExist, err := doesKeyPathExist(registry.LOCAL_MACHINE, groupPolicyNRPTParentKey)
	if err != nil {
		return trace.Wrap(err, "checking existence of group policy NRPT registry key %s", groupPolicyNRPTParentKey)
	}
	if !slices.Equal(cfg.dnsZones, state.configuredDNSZones) ||
		!slices.Equal(cfg.dnsAddrs, state.configuredDNSAddrs) ||
		doesGroupPolicyKeyExist != state.configuredGroupPolicyKey {
		if err := configureDNS(ctx, cfg.dnsZones, cfg.dnsAddrs, doesGroupPolicyKeyExist); err != nil {
			return trace.Wrap(err, "configuring DNS")
		}
		state.configuredDNSZones = cfg.dnsZones
		state.configuredDNSAddrs = cfg.dnsAddrs
		state.configuredGroupPolicyKey = doesGroupPolicyKeyExist
	}

	return nil
}

// addrMaskForCIDR returns the base address and the bitmask for a given CIDR
// range. The "route add" command wants the mask in dotted decimal format, e.g.
// for 100.64.0.0/10 the mask should be 255.192.0.0
func addrMaskForCIDR(cidr string) (string, string, error) {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", "", trace.Wrap(err, "parsing CIDR range %s", cidr)
	}
	return ipNet.IP.String(), maskForIPNet(ipNet), nil
}

func maskForIPNet(ipNet *net.IPNet) string {
	return net.IP(ipNet.Mask).String()
}

const (
	// Split DNS is configured via the Name Resolution Policy Table in the
	// Windows registry.

	// This key holds the local system NRPT configuration.
	systemNRPTParentKey = `SYSTEM\CurrentControlSet\Services\Dnscache\Parameters\DnsPolicyConfig`
	// This key holds NRPT entries coming from group policy, if it's present
	// then policies under systemNRPTParentKey are ignored, so VNet needs to write policies here.
	groupPolicyNRPTParentKey = `SOFTWARE\Policies\Microsoft\Windows NT\DNSClient\DnsPolicyConfig`
	// The UUID at the end was randomly generated once, VNet
	// always writes policies at this key and cleans it up on shutdown.
	vnetNRPTKeyID = `{ad074e9a-bd1b-447e-9108-14e545bf11a5}`
)

func configureDNS(ctx context.Context, zones, nameservers []string, doesGroupPolicyKeyExist bool) error {
	// Always configure NRPT rules under the local system NRPT registry key.
	// This is harmless/innefective if groupPolicyNRPTParentKey exists, but
	// always writing the rules here means they will be effective if
	// groupPolicyNRPTParentKey gets deleted later.
	nrptRegKey := systemNRPTParentKey + `\` + vnetNRPTKeyID
	if err := configureDNSAtNRPTKey(ctx, nrptRegKey, zones, nameservers); err != nil {
		return trace.Wrap(err, "configuring DNS NRPT at local system path %s", nrptRegKey)
	}

	// If groupPolicyNRPTParentKey exists then all rules under
	// systemNRPTParentKey will be ignored and rules under
	// groupPolicyNRPTParentKey take precendence, so VNet needs to write rules
	// under this key as well.
	if !doesGroupPolicyKeyExist {
		// The group policy parent key doesn't exist, no need to write under it.
		return nil
	}

	nrptRegKey = groupPolicyNRPTParentKey + `\` + vnetNRPTKeyID
	if err := configureDNSAtNRPTKey(ctx, nrptRegKey, zones, nameservers); err != nil {
		return trace.Wrap(err, "configuring DNS NRPT at group policy path %s", nrptRegKey)
	}
	// In some cases, rules under groupPolicyKey don't seem to be picked up by the DNS client service
	// until the computer refreshes its policies. [1] A force refresh here ensures they're picked up
	// immediately. See also https://github.com/gravitational/teleport/issues/60468.
	// 1: https://github.com/tailscale/tailscale/issues/4607#issuecomment-1130586168
	if err := forceRefreshComputerPolicies(); err != nil {
		return trace.Wrap(err, "refreshing computer policies")
	}
	return nil
}

func doesKeyPathExist(k registry.Key, path string) (bool, error) {
	key, err := registry.OpenKey(k, path, registry.READ)
	if err != nil {
		if !errors.Is(err, registry.ErrNotExist) {
			return false, trace.Wrap(err, "opening registry key %s", path)
		}
		return false, nil
	}
	if err := key.Close(); err != nil {
		return true, trace.Wrap(err, "closing registry key %s", path)
	}
	return true, nil
}

func configureDNSAtNRPTKey(ctx context.Context, nrptRegKey string, zones, nameservers []string) (err error) {
	if len(nameservers) == 0 {
		// Can't handle any zones if there are no nameservers.
		zones = nil
	}
	log.InfoContext(ctx, "Configuring DNS.", "reg_key", nrptRegKey, "zones", zones, "nameservers", nameservers)

	if len(zones) == 0 {
		// Either we have no zones we want to handle (the user is not
		// currently logged in to any clusters) or VNet is shutting down. Either
		// way, delete the registry key.
		return trace.Wrap(deleteRegistryKey(nrptRegKey))
	}

	// Open the registry key where split DNS will be configured.
	dnsKey, _ /*alreadyExisted*/, err := registry.CreateKey(registry.LOCAL_MACHINE, nrptRegKey, registry.SET_VALUE)
	if err != nil {
		return trace.Wrap(err, "failed to open Windows registry key to configure split DNS")
	}
	defer func() {
		var (
			origErr             = err
			deleteErr, closeErr error
		)
		// If this function failed for any reason, delete the registry key.
		if origErr != nil {
			deleteErr = trace.Wrap(deleteRegistryKey(nrptRegKey))
		}
		// Always close the registry key.
		closeErr = trace.Wrap(dnsKey.Close(), "closing DNS registry key")
		// Set the named return parameter [err] to the aggregate of all errors.
		err = trace.NewAggregate(origErr, deleteErr, closeErr)
	}()

	// The NRPT version must be 1.
	if err := dnsKey.SetDWordValue("Version", 1); err != nil {
		return trace.Wrap(err, "failed to set Version in DNS registry key")
	}
	// Name is a list of strings holding the DNS suffixes to match.
	// https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-gpnrpt/c1f8a4c0-d4e0-49b2-b4ef-87031be16662
	if err := dnsKey.SetStringsValue("Name", normalizeDNSZones(zones)); err != nil {
		return trace.Wrap(err, "failed to set Name in DNS registry key")
	}
	// GenericDNSServers is a string value holding a semicolon-delimited list of
	// IP addresses of DNS nameservers.
	// https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-gpnrpt/06088ca3-4cf1-48fa-8837-ca8d853ee1e8
	if err := dnsKey.SetStringValue("GenericDNSServers", strings.Join(nameservers, ";")); err != nil {
		return trace.Wrap(err, "failed to set GenericDNSServers in DNS registry key")
	}
	// Setting ConfigOptions to 8 tells NRPT that only GenericDNSServers is
	// specified (DNSSEC, DirectAccess, and IDN options are not set).
	// https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-gpnrpt/2d34f260-1e9e-4a52-ac91-2056dfd29702
	if err := dnsKey.SetDWordValue("ConfigOptions", 8); err != nil {
		return trace.Wrap(err, "failed to set ConfigOptions in DNS registry key")
	}

	return nil
}

func normalizeDNSZones(zones []string) []string {
	// For the registry key, zones must start with . and must not end with .
	out := make([]string, len(zones))
	for i := range out {
		out[i] = "." + strings.TrimSuffix(strings.TrimPrefix(zones[i], "."), ".")
	}
	return out
}

func deleteRegistryKey(key string) error {
	deleteErr := registry.DeleteKey(registry.LOCAL_MACHINE, key)
	if deleteErr == nil {
		// Successfully deleted the key.
		return nil
	}
	// Ignore the error if we also can't open the key, meaning it probably
	// doesn't exist.
	keyHandle, openErr := registry.OpenKey(registry.LOCAL_MACHINE, key, registry.READ)
	if openErr != nil {
		return nil
	}
	keyHandle.Close()
	return trace.Wrap(deleteErr, "failed to delete DNS registry key %s", key)
}

// https://learn.microsoft.com/en-us/windows/win32/api/userenv/nf-userenv-refreshpolicyex
func forceRefreshComputerPolicies() error {
	// refreshComputerPolicies corresponds to the first argument of RefreshPolicyEx which specifies
	// whether to refresh computer or user policies.
	const refreshComputerPolicies = 1
	// rpForce corresponds to the RP_FORCE flag for RefreshPolicyEx which makes it reapply all
	// policies even if no policy change was detected.
	const rpForce = 1

	retVal, _, err := procRefreshPolicyEx.Call(
		uintptr(refreshComputerPolicies),
		uintptr(rpForce),
	)
	if retVal == 0 {
		return trace.Wrap(err)
	}
	return nil
}
