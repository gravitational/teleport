/*
Copyright 2022 Gravitational, Inc.

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

package types

import (
	"slices"
	"strings"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/utils"
)

// Match checks if the given instance appears to match this filter.
func (f InstanceFilter) Match(i Instance) bool {
	if f.ServerID != "" && f.ServerID != i.GetName() {
		return false
	}

	if f.Version != "" && f.Version != i.GetTeleportVersion() {
		// TODO(fspmarshall): move some of the lib/versioncontrol helpers to
		// the api package and finalize version matching syntax so that we
		// can do normalization and wildcard matching.
		return false
	}

	if fv, ok := parseVersionRelaxed(f.OlderThanVersion); ok {
		if iv, ok := parseVersionRelaxed(i.GetTeleportVersion()); ok {
			if !iv.LessThan(fv) {
				return false
			}
		}
	}

	if fv, ok := parseVersionRelaxed(f.NewerThanVersion); ok {
		iv, ok := parseVersionRelaxed(i.GetTeleportVersion())

		if !ok {
			// treat instances with invalid versions are less/older than
			// valid versions.
			return false
		}

		if !fv.LessThan(iv) {
			return false
		}
	}

	// if Services was specified, ensure instance has at least one of the listed services.
	if len(f.Services) != 0 && slices.IndexFunc(f.Services, i.HasService) == -1 {
		return false
	}

	if f.ExternalUpgrader != "" && f.ExternalUpgrader != i.GetExternalUpgrader() {
		return false
	}

	// empty upgrader matches all, so we have a separate bool flag for
	// specifically matching instances with no ext upgrader defined.
	if f.NoExtUpgrader && i.GetExternalUpgrader() != "" {
		return false
	}

	return true
}

// shorthandChars are expected characters in version shorthand (e.g. "1" or "1.0" are shorthand for "1.0.0").
const shorthandChars = "0123456789."

// normalizeVersionShorthand attempts to convert go-style semver into the stricter semver
// notation expected by coreos/go-semver.
func normalizeVersionShorthand(version string) string {
	version = strings.TrimPrefix(version, "v")
	for _, c := range version {
		if !strings.ContainsRune(shorthandChars, c) {
			return version
		}
	}

	switch strings.Count(version, ".") {
	case 0:
		return version + ".0.0"
	case 1:
		return version + ".0"
	default:
		return version
	}
}

// parseVersionRelaxed wraps standard semver parsing with shorthand normalization.
func parseVersionRelaxed(version string) (ver semver.Version, ok bool) {
	if version == "" {
		return semver.Version{}, false
	}

	if ver.Set(normalizeVersionShorthand(version)) != nil {
		return semver.Version{}, false
	}

	return ver, true
}

// Instance describes the configuration/status of a unique teleport server identity. Each
// instance may be running one or more teleport services, and may have multiple processes
// associated with it.
type Instance interface {
	Resource

	// GetTeleportVersion gets the teleport version reported by the instance.
	GetTeleportVersion() string

	// GetServices gets the running services reported by the instance. This list is not
	// guaranteed to consist only of valid teleport services. Invalid/unexpected services
	// should be ignored.
	GetServices() []SystemRole

	// HasService checks if this instance advertises the specified service.
	HasService(SystemRole) bool

	// GetHostname gets the hostname reported by the instance.
	GetHostname() string

	// GetAuthID gets the server ID of the auth server that most recently reported
	// having observed this instance.
	GetAuthID() string

	// GetLastSeen gets the most recent time that an auth server reported having
	// seen this instance.
	GetLastSeen() time.Time

	// SetLastSeen sets the most recent time that an auth server reported having
	// seen this instance. Generally, if this value is being updated, the caller
	// should follow up by calling SyncLogAndResourceExpiry so that the control log
	// and resource-level expiry values can be reevaluated.
	SetLastSeen(time.Time)

	// GetExternalUpgrader gets the upgrader value as represented in the most recent
	// hello message from this instance. This value corresponds to the TELEPORT_EXT_UPGRADER
	// env var that is set when agents are configured to export schedule values to external
	// upgraders.
	GetExternalUpgrader() string

	// GetExternalUpgraderVersion gets the reported upgrader version. This value corresponds
	// to the TELEPORT_EXT_UPGRADER_VERSION env var that is set when agents are configured.
	GetExternalUpgraderVersion() string

	// SyncLogAndResourceExpiry filters expired entries from the control log and updates
	// the resource-level expiry. All calculations are performed relative to the value of
	// the LastSeen field, and the supplied TTL is used only as a default. The actual TTL
	// of an instance resource may be longer than the supplied TTL if one or more control
	// log entries use a custom TTL.
	SyncLogAndResourceExpiry(ttl time.Duration)

	// GetControlLog gets the instance control log entries associated with this instance.
	// The control log is a log of recent events related to an auth server's administration
	// of an instance's state. Auth servers generally ensure that they have successfully
	// written to the log *prior* to actually attempting the planned action. As a result,
	// the log may contain things that never actually happened.
	GetControlLog() []InstanceControlLogEntry

	// AppendControlLog appends entries to the control log. The control log is sorted by time,
	// so appends do not need to be performed in any particular order.
	AppendControlLog(entries ...InstanceControlLogEntry)

	// GetLastMeasurement returns information about the system clocks of the auth service and
	// another instance.
	GetLastMeasurement() *SystemClockMeasurement

	// Clone performs a deep copy on this instance.
	Clone() Instance
}

// NewInstance assembles a new instance resource.
func NewInstance(serverID string, spec InstanceSpecV1) (Instance, error) {
	instance := &InstanceV1{
		ResourceHeader: ResourceHeader{
			Metadata: Metadata{
				Name: serverID,
			},
		},
		Spec: spec,
	}
	if err := instance.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return instance, nil
}

func (i *InstanceV1) CheckAndSetDefaults() error {
	i.setStaticFields()
	if err := i.ResourceHeader.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if i.Version != V1 {
		return trace.BadParameter("unsupported instance resource version: %s", i.Version)
	}

	if i.Kind != KindInstance {
		return trace.BadParameter("unexpected resource kind: %q (expected %s)", i.Kind, KindInstance)
	}

	if i.Metadata.Namespace != "" && i.Metadata.Namespace != defaults.Namespace {
		return trace.BadParameter("invalid namespace %q (namespaces are deprecated)", i.Metadata.Namespace)
	}

	return nil
}

func (i *InstanceV1) setStaticFields() {
	if i.Version == "" {
		i.Version = V1
	}

	if i.Kind == "" {
		i.Kind = KindInstance
	}
}

func (i *InstanceV1) SyncLogAndResourceExpiry(ttl time.Duration) {
	// expire control log entries relative to LastSeen.
	logExpiry := i.expireControlLog(i.Spec.LastSeen, ttl)

	// calculate the default resource expiry.
	resourceExpiry := i.Spec.LastSeen.Add(ttl)

	// if one or more log entries want to outlive the default resource
	// expiry, we bump the resource expiry to match.
	if logExpiry.After(resourceExpiry) {
		resourceExpiry = logExpiry
	}

	i.Metadata.SetExpiry(resourceExpiry.UTC())
}

func (i *InstanceV1) GetTeleportVersion() string {
	return i.Spec.Version
}

func (i *InstanceV1) GetServices() []SystemRole {
	return i.Spec.Services
}

func (i *InstanceV1) HasService(s SystemRole) bool {
	return slices.Contains(i.Spec.Services, s)
}

func (i *InstanceV1) GetHostname() string {
	return i.Spec.Hostname
}

func (i *InstanceV1) GetAuthID() string {
	return i.Spec.AuthID
}

func (i *InstanceV1) GetLastSeen() time.Time {
	return i.Spec.LastSeen
}

func (i *InstanceV1) SetLastSeen(t time.Time) {
	i.Spec.LastSeen = t.UTC()
}

func (i *InstanceV1) GetExternalUpgrader() string {
	return i.Spec.ExternalUpgrader
}

func (i *InstanceV1) GetExternalUpgraderVersion() string {
	return i.Spec.ExternalUpgraderVersion
}

func (i *InstanceV1) GetControlLog() []InstanceControlLogEntry {
	return i.Spec.ControlLog
}

func (i *InstanceV1) AppendControlLog(entries ...InstanceControlLogEntry) {
	n := len(i.Spec.ControlLog)
	i.Spec.ControlLog = append(i.Spec.ControlLog, entries...)
	for idx, entry := range i.Spec.ControlLog[n:] {
		// ensure that all provided timestamps are UTC (non-UTC timestamps can cause
		// panics in proto logic).
		i.Spec.ControlLog[idx].Time = entry.Time.UTC()
	}
	slices.SortFunc(i.Spec.ControlLog, func(a, b InstanceControlLogEntry) int {
		return a.Time.Compare(b.Time)
	})
}

func (i *InstanceV1) GetLastMeasurement() *SystemClockMeasurement {
	return i.Spec.LastMeasurement
}

// expireControlLog removes expired entries from the control log relative to the supplied
// "now" value. The supplied ttl is used as the default ttl for entries that do not specify
// a custom ttl value. The returned timestamp is the observed expiry that was furthest in
// the future.
func (i *InstanceV1) expireControlLog(now time.Time, ttl time.Duration) time.Time {
	now = now.UTC()
	filtered := i.Spec.ControlLog[:0]
	var latestExpiry time.Time
	for _, entry := range i.Spec.ControlLog {
		entryTTL := entry.TTL
		if entryTTL == 0 {
			entryTTL = ttl
		}
		if entry.Time.IsZero() {
			entry.Time = now
		}
		expiry := entry.Time.Add(entryTTL)
		if now.After(expiry) {
			continue
		}

		if expiry.After(latestExpiry) {
			latestExpiry = expiry
		}
		filtered = append(filtered, entry)
	}
	// ensure that we don't preserve pointers in the now out of
	// range portion of the control log by zeroing the diff.
	for idx := len(filtered); idx < len(i.Spec.ControlLog); idx++ {
		i.Spec.ControlLog[idx] = InstanceControlLogEntry{}
	}
	i.Spec.ControlLog = filtered
	return latestExpiry
}

func (i *InstanceV1) Clone() Instance {
	return utils.CloneProtoMsg(i)
}

func (e *InstanceControlLogEntry) Clone() InstanceControlLogEntry {
	e.Time = e.Time.UTC()
	return *utils.CloneProtoMsg(e)
}
