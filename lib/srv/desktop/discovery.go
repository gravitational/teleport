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
	"maps"
	"net"
	"net/netip"
	"strings"
	"time"

	"github.com/go-ldap/ldap/v3"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/readonly"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/winpki"
)

const (
	// attrName is the name of an LDAP object.
	attrName = "name"

	// attrCommonName is the common name of an LDAP object, or "CN".
	attrCommonName = "cn"

	// attrObjectGUID is the globally unique identifier for an LDAP object.
	attrObjectGUID = "objectGUID"

	// attrDistinguishedName is the distinguished name of an LDAP object, or "DN".
	attrDistinguishedName = "distinguishedName"

	// attrPrimaryGroupID is the primary group ID of an LDAP object.
	attrPrimaryGroupID = "primaryGroupID"

	// attrOS is the operating system of a computer object
	attrOS = "operatingSystem"

	// attrOSVersion is the operating system version of a computer object.
	attrOSVersion = "operatingSystemVersion"

	// attrDNSHostName is the DNS Host name of an LDAP object.
	attrDNSHostName = "dNSHostName" // unusual capitalization is correct

	// attrSAMAccountName is the SAM Account name of an LDAP object.
	attrSAMAccountName = "sAMAccountName"

	// attrSAMAccountType is the SAM Account type for an LDAP object.
	attrSAMAccountType = "sAMAccountType"
)

const (
	// AccountTypeUser is the SAM account type for user accounts.
	// See https://learn.microsoft.com/en-us/windows/win32/adschema/a-samaccounttype
	// (SAM_USER_OBJECT)
	AccountTypeUser = "805306368"

	// ClassComputer is the object class for computers in Active Directory.
	ClassComputer = "computer"

	// ClassGMSA is the object class for group managed service accounts in Active Directory.
	ClassGMSA = "msDS-GroupManagedServiceAccount"
)

// See: https://docs.microsoft.com/en-US/windows/security/identity-protection/access-control/security-identifiers
const (
	// writableDomainControllerGroupID is the windows security identifier for dcs with write permissions
	writableDomainControllerGroupID = "516"
	// readOnlyDomainControllerGroupID is the windows security identifier for read only dcs
	readOnlyDomainControllerGroupID = "521"
)

// startDesktopDiscovery starts fetching desktops from LDAP, periodically
// registering and unregistering them as necessary.
func (s *WindowsService) startDesktopDiscovery() error {
	reconciler, err := services.NewReconciler(services.ReconcilerConfig[types.WindowsDesktop]{
		// Use a matcher that matches all resources, since our desktops are
		// pre-filtered by nature of using an LDAP search with filters.
		Matcher:             func(d types.WindowsDesktop) bool { return true },
		GetCurrentResources: func() map[string]types.WindowsDesktop { return s.lastDiscoveryResults },
		GetNewResources:     s.getDesktopsFromLDAP,
		OnCreate:            s.upsertDesktop,
		OnUpdate:            s.updateDesktop,
		OnDelete:            s.deleteDesktop,
		Logger:              s.cfg.Logger.With("kind", types.KindWindowsDesktop),
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

		t := s.cfg.Clock.NewTicker(s.cfg.DiscoveryInterval)
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

func (s *WindowsService) ldapSearchFilter(additionalFilters []string) string {
	var filters = []string{
		fmt.Sprintf("(%s=%s)", winpki.AttrObjectClass, ClassComputer),
		fmt.Sprintf("(!(%s=%s))", winpki.AttrObjectClass, ClassGMSA),
	}
	filters = append(filters, additionalFilters...)
	return winpki.CombineLDAPFilters(filters)
}

// getDesktopsFromLDAP discovers Windows hosts via LDAP
func (s *WindowsService) getDesktopsFromLDAP() map[string]types.WindowsDesktop {
	// Check whether we've ever successfully initialized our LDAP client.
	s.mu.Lock()
	if !s.ldapInitialized {
		s.cfg.Logger.DebugContext(s.closeCtx, "LDAP not ready, skipping discovery and attempting to reconnect")
		s.mu.Unlock()
		s.initializeLDAP()
		return nil
	}
	s.mu.Unlock()

	result := make(map[string]types.WindowsDesktop)
	for _, discoveryConfig := range s.cfg.Discovery {
		filter := s.ldapSearchFilter(discoveryConfig.Filters)
		s.cfg.Logger.DebugContext(s.closeCtx, "searching for desktops", "filter", filter)

		var attrs []string
		attrs = append(attrs, computerAttributes...)
		attrs = append(attrs, discoveryConfig.LabelAttributes...)

		entries, err := s.lc.ReadWithFilter(discoveryConfig.BaseDN, filter, attrs)
		if trace.IsConnectionProblem(err) {
			// If the connection was broken, re-initialize the LDAP client so that it's
			// ready for the next reconcile loop. Return the last known set of desktops
			// in this case, so that the reconciler doesn't delete the desktops it already
			// knows about.
			s.cfg.Logger.InfoContext(s.closeCtx, "LDAP connection error when searching for desktops, reinitializing client")
			if err := s.initializeLDAP(); err != nil {
				s.cfg.Logger.ErrorContext(s.closeCtx, "failed to reinitialize LDAP client, will retry on next reconcile", "error", err)
			}
			return s.lastDiscoveryResults
		} else if err != nil {
			s.cfg.Logger.WarnContext(s.closeCtx, "could not discover Windows Desktops", "error", err)
			return nil
		}

		s.cfg.Logger.DebugContext(s.closeCtx, "discovered Windows Desktops", "count", len(entries))

		for _, entry := range entries {
			desktop, err := s.ldapEntryToWindowsDesktop(s.closeCtx, entry, s.cfg.HostLabelsFn, &discoveryConfig)
			if err != nil {
				s.cfg.Logger.WarnContext(s.closeCtx, "could not create Windows Desktop from LDAP entry", "error", err)
				continue
			}
			result[desktop.GetName()] = desktop
		}
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

func (s *WindowsService) applyLabelsFromLDAP(entry *ldap.Entry, labels map[string]string, cfg *servicecfg.LDAPDiscoveryConfig) {
	// apply common LDAP labels by default
	labels[types.OriginLabel] = types.OriginDynamic
	labels[types.DiscoveryLabelWindowsDNSHostName] = entry.GetAttributeValue(attrDNSHostName)
	labels[types.DiscoveryLabelWindowsComputerName] = entry.GetAttributeValue(attrName)
	labels[types.DiscoveryLabelWindowsOS] = entry.GetAttributeValue(attrOS)
	labels[types.DiscoveryLabelWindowsOSVersion] = entry.GetAttributeValue(attrOSVersion)

	// attempt to compute the desktop's OU from its DN
	dn := entry.GetAttributeValue(attrDistinguishedName)
	cn := entry.GetAttributeValue(attrCommonName)
	if len(dn) > 0 && len(cn) > 0 {
		ou := strings.TrimPrefix(dn, "CN="+cn+",")
		labels[types.DiscoveryLabelWindowsOU] = ou
	}

	// label domain controllers
	switch entry.GetAttributeValue(attrPrimaryGroupID) {
	case writableDomainControllerGroupID, readOnlyDomainControllerGroupID:
		labels[types.DiscoveryLabelWindowsIsDomainController] = "true"
	}

	// apply any custom labels per the discovery configuration
	for _, attr := range cfg.LabelAttributes {
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
func (s *WindowsService) ldapEntryToWindowsDesktop(
	ctx context.Context,
	entry *ldap.Entry,
	getHostLabels func(string) map[string]string,
	cfg *servicecfg.LDAPDiscoveryConfig,
) (types.WindowsDesktop, error) {
	hostname := entry.GetAttributeValue(attrDNSHostName)
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
	s.applyLabelsFromLDAP(entry, labels, cfg)

	if os, ok := labels[types.DiscoveryLabelWindowsOS]; ok && strings.Contains(os, "linux") {
		return nil, trace.BadParameter("LDAP entry looks like a Linux host")
	}

	addrs, err := s.lookupDesktop(ctx, hostname)
	if err != nil || len(addrs) == 0 {
		return nil, trace.WrapWithMessage(err, "couldn't resolve %q", hostname)
	}

	s.cfg.Logger.DebugContext(ctx, "resolved desktop host", "hostname", hostname, "addrs", addrs)
	addr, err := utils.ParseHostPortAddr(addrs[0], cfg.RDPPort)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// ensure no '.' in name, because we use SNI to route to the right
	// desktop, and our cert is valid for *.desktop.teleport.cluster.local
	name := strings.ReplaceAll(hostname, ".", "-")

	// append portion of the object GUID to ensure that desktops from
	// different domains that happen to have the same hostname don't conflict
	if guid := entry.GetRawAttributeValue(attrObjectGUID); len(guid) >= 4 {
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

// startDynamicReconciler starts resource watcher and reconciler that registers/unregisters Windows desktops
// according to the up-to-date list of dynamic Windows desktops resources.
func (s *WindowsService) startDynamicReconciler(ctx context.Context) (*services.GenericWatcher[types.DynamicWindowsDesktop, readonly.DynamicWindowsDesktop], error) {
	if len(s.cfg.ResourceMatchers) == 0 {
		s.cfg.Logger.DebugContext(ctx, "Not starting dynamic desktop resource watcher.")
		return nil, nil
	}
	s.cfg.Logger.DebugContext(ctx, "Starting dynamic desktop resource watcher.")
	dynamicDesktopClient := s.cfg.AuthClient.DynamicDesktopClient()
	watcher, err := services.NewDynamicWindowsDesktopWatcher(ctx, services.DynamicWindowsDesktopWatcherConfig{
		DynamicWindowsDesktopGetter: dynamicDesktopClient,
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentWindowsDesktop,
			Client:    s.cfg.AccessPoint,
		},
	})

	if err != nil {
		return nil, trace.Wrap(err)
	}

	currentResources := make(map[string]types.WindowsDesktop)
	var newResources map[string]types.WindowsDesktop

	reconciler, err := services.NewReconciler(services.ReconcilerConfig[types.WindowsDesktop]{
		Matcher: func(desktop types.WindowsDesktop) bool {
			return services.MatchResourceLabels(s.cfg.ResourceMatchers, desktop.GetAllLabels())
		},
		GetCurrentResources: func() map[string]types.WindowsDesktop {
			maps.DeleteFunc(currentResources, func(_ string, v types.WindowsDesktop) bool {
				d, err := s.cfg.AuthClient.GetWindowsDesktops(ctx, types.WindowsDesktopFilter{
					HostID: v.GetHostID(),
					Name:   v.GetName(),
				})
				return err != nil || len(d) == 0
			})
			return currentResources
		},
		GetNewResources: func() map[string]types.WindowsDesktop {
			return newResources
		},
		OnCreate: s.upsertDesktop,
		OnUpdate: s.updateDesktop,
		OnDelete: s.deleteDesktop,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	go func() {
		defer s.cfg.Logger.DebugContext(ctx, "DynamicWindowsDesktop resource watcher done.")
		defer watcher.Close()
		tickDuration := 5 * time.Minute
		expiryDuration := tickDuration + 2*time.Minute
		tick := s.cfg.Clock.NewTicker(tickDuration)
		defer tick.Stop()
		for {
			select {
			case desktops := <-watcher.ResourcesC:
				newResources = make(map[string]types.WindowsDesktop)
				for _, dynamicDesktop := range desktops {
					desktop, err := s.toWindowsDesktop(dynamicDesktop)
					desktop.SetExpiry(s.cfg.Clock.Now().Add(expiryDuration))
					if err != nil {
						s.cfg.Logger.WarnContext(ctx, "Can't create desktop resource", "error", err)
						continue
					}
					newResources[dynamicDesktop.GetName()] = desktop
				}
				if err := reconciler.Reconcile(ctx); err != nil {
					s.cfg.Logger.WarnContext(ctx, "Reconciliation failed, will retry", "error", err)
					continue
				}
				currentResources = newResources
			case <-tick.Chan():
				newResources = make(map[string]types.WindowsDesktop)
				for k, v := range currentResources {
					newResources[k] = v.Copy()
					newResources[k].SetExpiry(s.cfg.Clock.Now().Add(expiryDuration))
				}
				if err := reconciler.Reconcile(ctx); err != nil {
					s.cfg.Logger.WarnContext(ctx, "Reconciliation failed, will retry", "error", err)
					continue
				}
				currentResources = newResources
			case <-watcher.Done():
				return
			case <-ctx.Done():
				return
			}
		}
	}()
	return watcher, nil
}

func (s *WindowsService) toWindowsDesktop(dynamicDesktop types.DynamicWindowsDesktop) (*types.WindowsDesktopV3, error) {
	width, height := dynamicDesktop.GetScreenSize()
	desktopLabels := dynamicDesktop.GetAllLabels()
	labels := make(map[string]string, len(desktopLabels)+1)
	maps.Copy(labels, desktopLabels)
	labels[types.OriginLabel] = types.OriginDynamic
	return types.NewWindowsDesktopV3(dynamicDesktop.GetName(), labels, types.WindowsDesktopSpecV3{
		Addr:   dynamicDesktop.GetAddr(),
		Domain: dynamicDesktop.GetDomain(),
		HostID: s.cfg.Heartbeat.HostUUID,
		NonAD:  dynamicDesktop.NonAD(),
		ScreenSize: &types.Resolution{
			Width:  width,
			Height: height,
		},
	})
}
