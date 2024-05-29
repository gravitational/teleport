package vnet

import (
	"context"
	"log/slog"
	"net"
	"slices"
	"sync"

	"github.com/gravitational/trace"
)

const (
	clusterTXTRecordPrefix = "teleport-cluster="
)

type lookupTXTFunc = func(ctx context.Context, domain string) (txtRecords []string, err error)

// customDNSZoneValidator validates custom DNS zones by making sure that they have a DNS TXT record of
// "teleport-cluster=<cluster-name>" for the cluster in which they are used. This is meant to avoid the
// possibility of a rogue application advertising a public_addr with a DNS name not controlled by the cluster
// admins, which could be used to trick VNet users.
//
// After finding that a zone is valid once, this is cached indefinitely. Invalid zones are a misconfiguration
// so we don't cache negative results
type customDNSZoneValidator struct {
	lookupTXT  lookupTXTFunc
	validZones map[string]struct{}
	mu         sync.RWMutex
}

func newCustomDNSZoneValidator(lookupTXT lookupTXTFunc) *customDNSZoneValidator {
	if lookupTXT == nil {
		var resolver net.Resolver
		lookupTXT = resolver.LookupTXT
	}
	return &customDNSZoneValidator{
		lookupTXT:  lookupTXT,
		validZones: make(map[string]struct{}),
	}
}

// validate returns an error if [customDNSZone] is not valid for [clusterName].
func (c *customDNSZoneValidator) validate(ctx context.Context, clusterName, customDNSZone string) error {
	c.mu.RLock()
	_, ok := c.validZones[customDNSZone]
	c.mu.RUnlock()
	if ok {
		return nil
	}

	requiredTXTRecord := clusterTXTRecordPrefix + clusterName
	slog.InfoContext(ctx, "Checking validity of custom DNS zone by querying for required TXT record.", "zone", customDNSZone, "record", requiredTXTRecord)

	records, err := c.lookupTXT(ctx, customDNSZone)
	if err != nil {
		return trace.Wrap(err, "looking up TXT records for %q", customDNSZone)
	}

	valid := slices.Contains(records, requiredTXTRecord)
	if !valid {
		return trace.BadParameter(`custom DNS zone %q does not have required TXT record %q`, customDNSZone, requiredTXTRecord)
	}

	slog.DebugContext(ctx, "Custom DNS zone has valid TXT record.", "zone", customDNSZone, "cluster", clusterName)

	c.mu.Lock()
	defer c.mu.Unlock()
	c.validZones[customDNSZone] = struct{}{}
	return nil
}
