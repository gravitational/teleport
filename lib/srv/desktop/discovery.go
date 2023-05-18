// Copyright 2021 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package desktop

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/go-ldap/ldap/v3"
	"github.com/gravitational/trace"

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
	reconciler, err := services.NewReconciler(services.ReconcilerConfig{
		// Use a matcher that matches all resources, since our desktops are
		// pre-filtered by nature of using an LDAP search with filters.
		Matcher: func(r types.ResourceWithLabels) bool { return true },

		GetCurrentResources: func() types.ResourcesWithLabelsMap { return s.lastDiscoveryResults },
		GetNewResources:     s.getDesktopsFromLDAP,
		OnCreate:            s.upsertDesktop,
		OnUpdate:            s.upsertDesktop,
		OnDelete:            s.deleteDesktop,
		Log:                 s.cfg.Log,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	go func() {
		// reconcile once before starting the ticker, so that desktops show up immediately
		// (we still have a small delay to give the LDAP client time to initialize)
		time.Sleep(15 * time.Second)
		if err := reconciler.Reconcile(s.closeCtx); err != nil && err != context.Canceled {
			s.cfg.Log.Errorf("desktop reconciliation failed: %v", err)
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
				if err := reconciler.Reconcile(s.closeCtx); err != nil && err != context.Canceled {
					s.cfg.Log.Errorf("desktop reconciliation failed: %v", err)
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
func (s *WindowsService) getDesktopsFromLDAP() types.ResourcesWithLabelsMap {
	if !s.ldapReady() {
		s.cfg.Log.Warn("skipping desktop discovery: LDAP not yet initialized")
		return nil
	}

	filter := s.ldapSearchFilter()
	s.cfg.Log.Debugf("searching for desktops with LDAP filter %v", filter)

	var attrs []string
	attrs = append(attrs, ComputerAttributes...)
	attrs = append(attrs, s.cfg.DiscoveryLDAPAttributeLabels...)

	entries, err := s.lc.ReadWithFilter(s.cfg.DiscoveryBaseDN, filter, attrs)
	if trace.IsConnectionProblem(err) {
		// If the connection was broken, re-initialize the LDAP client so that it's
		// ready for the next reconcile loop. Return the last known set of desktops
		// in this case, so that the reconciler doesn't delete the desktops it already
		// knows about.
		s.cfg.Log.Info("LDAP connection error when searching for desktops, reinitializing client")
		if err := s.initializeLDAP(); err != nil {
			s.cfg.Log.Errorf("failed to reinitialize LDAP client, will retry on next reconcile: %v", err)
		}
		return s.lastDiscoveryResults
	} else if err != nil {
		s.cfg.Log.Warnf("could not discover Windows Desktops: %v", err)
		return nil
	}

	s.cfg.Log.Debugf("discovered %d Windows Desktops", len(entries))

	result := make(types.ResourcesWithLabelsMap)
	for _, entry := range entries {
		desktop, err := s.ldapEntryToWindowsDesktop(s.closeCtx, entry, s.cfg.HostLabelsFn)
		if err != nil {
			s.cfg.Log.Warnf("could not create Windows Desktop from LDAP entry: %v", err)
			continue
		}
		result[desktop.GetName()] = desktop
	}

	// capture the result, which will be used on the next reconcile loop
	s.lastDiscoveryResults = result

	return result
}

func (s *WindowsService) upsertDesktop(ctx context.Context, r types.ResourceWithLabels) error {
	d, ok := r.(types.WindowsDesktop)
	if !ok {
		return trace.Errorf("upsert: expected a WindowsDesktop, got %T", r)
	}
	return s.cfg.AuthClient.UpsertWindowsDesktop(ctx, d)
}

func (s *WindowsService) deleteDesktop(ctx context.Context, r types.ResourceWithLabels) error {
	d, ok := r.(types.WindowsDesktop)
	if !ok {
		return trace.Errorf("delete: expected a WindowsDesktop, got %T", r)
	}
	return s.cfg.AuthClient.DeleteWindowsDesktop(ctx, d.GetHostID(), d.GetName())
}

func (s *WindowsService) applyLabelsFromLDAP(entry *ldap.Entry, labels map[string]string) {
	// apply common LDAP labels by default
	labels[types.OriginLabel] = types.OriginDynamic
	labels[types.TeleportNamespace+"/dns_host_name"] = entry.GetAttributeValue(windows.AttrDNSHostName)
	labels[types.TeleportNamespace+"/computer_name"] = entry.GetAttributeValue(windows.AttrName)
	labels[types.TeleportNamespace+"/os"] = entry.GetAttributeValue(windows.AttrOS)
	labels[types.TeleportNamespace+"/os_version"] = entry.GetAttributeValue(windows.AttrOSVersion)

	// attempt to compute the desktop's OU from its DN
	dn := entry.GetAttributeValue(windows.AttrDistinguishedName)
	cn := entry.GetAttributeValue(windows.AttrCommonName)
	if len(dn) > 0 && len(cn) > 0 {
		ou := strings.TrimPrefix(dn, "CN="+cn+",")
		labels[types.TeleportNamespace+"/ou"] = ou
	}

	// label domain controllers
	switch entry.GetAttributeValue(windows.AttrPrimaryGroupID) {
	case windows.WritableDomainControllerGroupID, windows.ReadOnlyDomainControllerGroupID:
		labels[types.TeleportNamespace+"/is_domain_controller"] = "true"
	}

	// apply any custom labels per the discovery configuration
	for _, attr := range s.cfg.DiscoveryLDAPAttributeLabels {
		if v := entry.GetAttributeValue(attr); v != "" {
			labels["ldap/"+attr] = v
		}
	}
}

// lookupDesktop does a DNS lookup for the provided hostname.
// It checks using the default system resolver first, and falls
// back to making a DNS query of the configured LDAP server
// if the system resolver fails.
func (s *WindowsService) lookupDesktop(ctx context.Context, hostname string) (addrs []string, err error) {
	tctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	addrs, err = net.DefaultResolver.LookupHost(tctx, hostname)
	if err == nil && len(addrs) > 0 {
		return addrs, nil
	}
	if s.dnsResolver == nil {
		return nil, trace.NewAggregate(err, trace.Errorf("DNS lookup for %q failed and there's no LDAP server to fallback to", hostname))
	}
	s.cfg.Log.WithError(err).Debugf("DNS lookup for %q failed, falling back to LDAP server", hostname)
	return s.dnsResolver.LookupHost(ctx, hostname)
}

// ldapEntryToWindowsDesktop generates the Windows Desktop resource
// from an LDAP search result
func (s *WindowsService) ldapEntryToWindowsDesktop(ctx context.Context, entry *ldap.Entry, getHostLabels func(string) map[string]string) (types.ResourceWithLabels, error) {
	hostname := entry.GetAttributeValue(windows.AttrDNSHostName)
	if hostname == "" {
		return nil, trace.BadParameter("LDAP entry missing hostname, has attributes: %v", entry.Attributes)
	}
	labels := getHostLabels(hostname)
	labels[types.TeleportNamespace+"/windows_domain"] = s.cfg.Domain
	s.applyLabelsFromLDAP(entry, labels)

	addrs, err := s.lookupDesktop(ctx, hostname)
	if err != nil || len(addrs) == 0 {
		return nil, trace.WrapWithMessage(err, "couldn't resolve %q", hostname)
	}

	s.cfg.Log.Debugf("resolved %v => %v", hostname, addrs)
	addr, err := utils.ParseHostPortAddr(addrs[0], defaults.RDPListenPort)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	desktop, err := types.NewWindowsDesktopV3(
		// ensure no '.' in name, because we use SNI to route to the right
		// desktop, and our cert is valid for *.desktop.teleport.cluster.local
		strings.ReplaceAll(hostname, ".", "-"),
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

	desktop.SetExpiry(s.cfg.Clock.Now().UTC().Add(apidefaults.ServerAnnounceTTL))
	return desktop, nil
}
