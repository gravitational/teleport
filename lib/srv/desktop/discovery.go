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

package desktop

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"time"

	"github.com/go-ldap/ldap/v3"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/windows"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

// startDesktopDiscovery starts fetching desktops from LDAP, periodically
// registering and unregistering them as necessary.
func (s *WindowsService) startDesktopDiscovery() error {
	reconciler, err := services.NewReconciler(services.ReconcilerConfig[types.WindowsDesktop]{
		// Use a matcher that matches all resources, since our desktops are
		// pre-filtered by nature of using an LDAP search with filters.
		Matcher: func(d types.WindowsDesktop) bool { return true },

		GetCurrentResources: func() map[string]types.WindowsDesktop { return s.lastDiscoveryResults },
		GetNewResources:     s.getDesktopsFromLDAP,
		OnCreate:            s.upsertDesktop,
		OnUpdate:            s.updateDesktop,
		OnDelete:            s.deleteDesktop,
		Log:                 logrus.NewEntry(logrus.StandardLogger()),
	})
	if err != nil {
		return trace.Wrap(err)
	}

	go func() {
		// reconcile once before starting the ticker, so that desktops show up immediately
		// (we still have a small delay to give the LDAP client time to initialize)
		time.Sleep(15 * time.Second)
		if err := reconciler.Reconcile(s.closeCtx); err != nil && !errors.Is(err, context.Canceled) {
			s.cfg.Logger.ErrorContext(s.closeCtx, "desktop reconciliation failed", "error", err)
		}

		// TODO(zmb3): consider making the discovery period configurable
		// (it's currently hard coded to 5 minutes in order to match DB access discovery behavior)
		t := s.cfg.Clock.NewTicker(5 * time.Minute)
		defer t.Stop()
		for {
			select {
			case <-s.closeCtx.Done():
				return
			case <-t.Chan():
				if err := reconciler.Reconcile(s.closeCtx); err != nil && !errors.Is(err, context.Canceled) {
					s.cfg.Logger.ErrorContext(s.closeCtx, "desktop reconciliation failed", "error", err)
				}
			}
		}
	}()

	return nil
}

func (s *WindowsService) ldapSearchFilter() string {
	var filters []string
	filters = append(filters, fmt.Sprintf("(%s=%s)", windows.AttrObjectClass, windows.ClassComputer))
	filters = append(filters, fmt.Sprintf("(!(%s=%s))", windows.AttrObjectClass, windows.ClassGMSA))
	filters = append(filters, s.cfg.DiscoveryLDAPFilters...)

	return windows.CombineLDAPFilters(filters)
}

// getDesktopsFromLDAP discovers Windows hosts via LDAP
func (s *WindowsService) getDesktopsFromLDAP() map[string]types.WindowsDesktop {
	if !s.ldapReady() {
		s.cfg.Logger.WarnContext(context.Background(), "skipping desktop discovery: LDAP not yet initialized")
		return nil
	}

	filter := s.ldapSearchFilter()
	s.cfg.Logger.DebugContext(context.Background(), "searching for desktops", "filter", filter)

	var attrs []string
	attrs = append(attrs, ComputerAttributes...)
	attrs = append(attrs, s.cfg.DiscoveryLDAPAttributeLabels...)

	entries, err := s.lc.ReadWithFilter(s.cfg.DiscoveryBaseDN, filter, attrs)
	if trace.IsConnectionProblem(err) {
		// If the connection was broken, re-initialize the LDAP client so that it's
		// ready for the next reconcile loop. Return the last known set of desktops
		// in this case, so that the reconciler doesn't delete the desktops it already
		// knows about.
		s.cfg.Logger.InfoContext(context.Background(), "LDAP connection error when searching for desktops, reinitializing client")
		if err := s.initializeLDAP(); err != nil {
			s.cfg.Logger.ErrorContext(context.Background(), "failed to reinitialize LDAP client, will retry on next reconcile", "error", err)
		}
		return s.lastDiscoveryResults
	} else if err != nil {
		s.cfg.Logger.WarnContext(context.Background(), "could not discover Windows Desktops", "error", err)
		return nil
	}

	s.cfg.Logger.DebugContext(context.Background(), "discovered Windows Desktops", "count", len(entries))

	result := make(map[string]types.WindowsDesktop)
	for _, entry := range entries {
		desktop, err := s.ldapEntryToWindowsDesktop(s.closeCtx, entry, s.cfg.HostLabelsFn)
		if err != nil {
			s.cfg.Logger.WarnContext(s.closeCtx, "could not create Windows Desktop from LDAP entry", "error", err)
			continue
		}
		result[desktop.GetName()] = desktop
	}

	// capture the result, which will be used on the next reconcile loop
	s.lastDiscoveryResults = result

	return result
}

func (s *WindowsService) updateDesktop(ctx context.Context, desktop, _ types.WindowsDesktop) error {
	return s.upsertDesktop(ctx, desktop)
}

func (s *WindowsService) upsertDesktop(ctx context.Context, d types.WindowsDesktop) error {
	return s.cfg.AuthClient.UpsertWindowsDesktop(ctx, d)
}

func (s *WindowsService) deleteDesktop(ctx context.Context, d types.WindowsDesktop) error {
	return s.cfg.AuthClient.DeleteWindowsDesktop(ctx, d.GetHostID(), d.GetName())
}

func (s *WindowsService) applyLabelsFromLDAP(entry *ldap.Entry, labels map[string]string) {
	// apply common LDAP labels by default
	labels[types.OriginLabel] = types.OriginDynamic
	labels[types.DiscoveryLabelWindowsDNSHostName] = entry.GetAttributeValue(windows.AttrDNSHostName)
	labels[types.DiscoveryLabelWindowsComputerName] = entry.GetAttributeValue(windows.AttrName)
	labels[types.DiscoveryLabelWindowsOS] = entry.GetAttributeValue(windows.AttrOS)
	labels[types.DiscoveryLabelWindowsOSVersion] = entry.GetAttributeValue(windows.AttrOSVersion)

	// attempt to compute the desktop's OU from its DN
	dn := entry.GetAttributeValue(windows.AttrDistinguishedName)
	cn := entry.GetAttributeValue(windows.AttrCommonName)
	if len(dn) > 0 && len(cn) > 0 {
		ou := strings.TrimPrefix(dn, "CN="+cn+",")
		labels[types.DiscoveryLabelWindowsOU] = ou
	}

	// label domain controllers
	switch entry.GetAttributeValue(windows.AttrPrimaryGroupID) {
	case windows.WritableDomainControllerGroupID, windows.ReadOnlyDomainControllerGroupID:
		labels[types.DiscoveryLabelWindowsIsDomainController] = "true"
	}

	// apply any custom labels per the discovery configuration
	for _, attr := range s.cfg.DiscoveryLDAPAttributeLabels {
		if v := entry.GetAttributeValue(attr); v != "" {
			labels[types.DiscoveryLabelLDAPPrefix+attr] = v
		}
	}
}

const dnsQueryTimeout = 5 * time.Second

// lookupDesktop does a DNS lookup for the provided hostname.
// It checks using the default system resolver first, and falls
// back to the configured LDAP server if the system resolver fails.
func (s *WindowsService) lookupDesktop(ctx context.Context, hostname string) ([]string, error) {
	stringAddrs := func(addrs []netip.Addr) []string {
		result := make([]string, 0, len(addrs))
		for _, addr := range addrs {
			result = append(result, addr.String())
		}
		return result
	}

	queryResolver := func(resolver *net.Resolver, resolverName string) chan []netip.Addr {
		ch := make(chan []netip.Addr, 1)
		go func() {
			tctx, cancel := context.WithTimeout(ctx, dnsQueryTimeout)
			defer cancel()

			addrs, err := resolver.LookupNetIP(tctx, "ip4", hostname)
			if err != nil {
				s.cfg.Logger.DebugContext(ctx, "DNS lookup failed", "hostname", hostname, "resolver", resolverName, "error", err)
			}

			// even though we requested "ip4" it's possible to get IPv4
			// addresses mapped to IPv6 addresses, so we unmap them here
			result := make([]netip.Addr, 0, len(addrs))
			for _, addr := range addrs {
				if addr.Is4() || addr.Is4In6() {
					result = append(result, addr.Unmap())
				}
			}

			ch <- result
		}()
		return ch
	}

	// kick off both DNS queries in parallel
	defaultResult := queryResolver(net.DefaultResolver, "default")
	ldapResult := queryResolver(s.dnsResolver, "LDAP")

	// wait for the default resolver to return (or time out)
	addrs := <-defaultResult
	if len(addrs) > 0 {
		return stringAddrs(addrs), nil
	}

	// If we didn't get a result from the default resolver,
	// use the result from the LDAP resolver.
	// This shouldn't block for very long, since both operations
	// started at the same time with the same timeout.
	addrs = <-ldapResult
	if len(addrs) > 0 {
		return stringAddrs(addrs), nil
	}

	return nil, trace.Errorf("could not resolve %v in time", hostname)
}

// ldapEntryToWindowsDesktop generates the Windows Desktop resource
// from an LDAP search result
func (s *WindowsService) ldapEntryToWindowsDesktop(ctx context.Context, entry *ldap.Entry, getHostLabels func(string) map[string]string) (types.WindowsDesktop, error) {
	hostname := entry.GetAttributeValue(windows.AttrDNSHostName)
	if hostname == "" {
		attrs := make([]string, len(entry.Attributes))
		for _, a := range entry.Attributes {
			attrs = append(attrs, fmt.Sprintf("%v=%v", a.Name, a.Values))
		}
		s.cfg.Logger.DebugContext(ctx, "LDAP entry is missing hostname", "dn", entry.DN, "attrs", strings.Join(attrs, ","))
		return nil, trace.BadParameter("LDAP entry %v missing hostname", entry.DN)
	}
	labels := getHostLabels(hostname)
	labels[types.DiscoveryLabelWindowsDomain] = s.cfg.Domain
	s.applyLabelsFromLDAP(entry, labels)

	addrs, err := s.lookupDesktop(ctx, hostname)
	if err != nil || len(addrs) == 0 {
		return nil, trace.WrapWithMessage(err, "couldn't resolve %q", hostname)
	}

	s.cfg.Logger.DebugContext(ctx, "resolved desktop host", "hostname", hostname, "addrs", addrs)
	addr, err := utils.ParseHostPortAddr(addrs[0], defaults.RDPListenPort)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// ensure no '.' in name, because we use SNI to route to the right
	// desktop, and our cert is valid for *.desktop.teleport.cluster.local
	name := strings.ReplaceAll(hostname, ".", "-")

	// append portion of the object GUID to ensure that desktops from
	// different domains that happen to have the same hostname don't conflict
	if guid := entry.GetRawAttributeValue(windows.AttrObjectGUID); len(guid) >= 4 {
		name += "-" + hex.EncodeToString(guid[:4])
	}

	desktop, err := types.NewWindowsDesktopV3(
		name,
		labels,
		types.WindowsDesktopSpecV3{
			Addr:   addr.String(),
			Domain: s.cfg.Domain,
			HostID: s.cfg.Heartbeat.HostUUID,
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// We use a longer TTL for discovered desktops, because the reconciler will manually
	// purge them if they stop being detected, and discovery of large Windows fleets can
	// take a long time.
	desktop.SetExpiry(s.cfg.Clock.Now().UTC().Add(apidefaults.ServerAnnounceTTL * 3))
	return desktop, nil
}
