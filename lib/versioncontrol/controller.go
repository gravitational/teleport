/*
Copyright 2021 Gravitational, Inc.

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

package versioncontrol

import (
	"time"

	vc "github.com/gravitational/teleport/api/versioncontrol"
	"github.com/gravitational/teleport/lib/inventory"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

// Status is an enum indicating the current version control status of a given instance.
type Status string

const (
	//
	StatusNone Status = ""
	// StatusVersionParity indicates that the server advertises the correct version (or that no version directive
	// matches the server) and that the server was not recently sent any install messages.
	StatusVersionParity Status = "version-parity"
	// StatusNeedsInstall indicates that the server advertises a different version than the one specified in its
	// matching version directive, and no recent install attempts have been made.
	StatusNeedsInstall Status = "needs-install"
	// StatusInstallTriggered indicates that an install attempt was triggered recently enough that it is unclear
	// what the result is (i.e. install can be thought of as 'in flight'). The triggering auth server may still
	// be planning to take additional actions.
	StatusInstallTriggered Status = "install-triggered"
	// StatusRecentInstall indicates that the server has recently sent a local install message, and is now
	// advertising a version matching the target of that message. This status is a kind of "backoff". If the
	// version directive changes, or the instance's labels change s.t. it matches a different directive, install
	// is not triggered until after the recency phase.
	StatusRecentInstall Status = "recent-install"
	// StatusChurnedDuringInstall indicates that the server appears to have gone offline immediately before, during,
	// or immediately after an installation. It is impossible to determine whether this was caused by the install
	// attempt, but for a given environment there is some portion/rate of churn that, if exceeded, is likely significant.
	StatusChurnedDuringInstall Status = "churned-during-install"
	// StatusImplicifInstallFault indicates that the server is online but seems to have failed to install the new version
	// for some reason. Its possible that the server never got the install message, or that it performed a full install
	// and rollback, but could not update its status for some reason.
	StatusImplicitInstallFault Status = "implicit-install-fault"
	// StatusExplicitInstallFault indicates that the server is online and seems to have failed to install the new version
	// for some reason, but has successfully emitted at least one error message.
	StatusExplicitInstallFault Status = "explicit-install-fault"
)

const (
	// LogEntryLocalInstallAttempt is the type of control log entry used to indicate a local install attempt.
	LogEntryLocalInstallAttempt = "local-install-attempt"

	// LogEntryLocalInstallFault is the type of control log entry used to indicate that an error was observed
	// during an install attempt.
	LogEntryLocalInstallFault = "local-install-fault"

	// LogLabelInstallerKind is the installer kind used in an install attempt.
	LogLabelInstallerKind = "installer-kind"

	// LogLabelInstallerName is the name of the installer used in an install attempt.
	LogLabelInstallerName = "installer-name"

	// LogLabelError is an error message associated with an install fault.
	LogLabelError = "error"
)

type Config struct {
	Service   services.VersionControl
	Inventory *inventory.Controller

	InstallGracePeriod time.Duration
	FaultBackoffPeriod time.Duration

	OfflineThreshold time.Duration
	ChurnedThreshold time.Duration
	Clock            clockwork.Clock
}

func (c *Config) CheckAndSetDefaults() error {
	if c.Service == nil {
		return trace.BadParameter("missing required parameter 'Service' for versioncontrol.Controller config")
	}

	if c.Inventory == nil {
		return trace.BadParameter("missing required parameter 'Inventory' for libvc.Controller config")
	}

	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}

	return nil
}

type Controller struct {
	cfg Config
}

func NewController(cfg Config) (*Controller, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &Controller{
		cfg: cfg,
	}, nil
}

type InstanceStatusSummary struct {
	Status    Status
	Target    vc.Target
	Installer types.VersionControlInstallerRef
	Error     string
}

// InstanceStateInfo is the information used to determine the version control status of an instance. We use this
// datastructure to abstract over directly connected instances, and instance resources in the backend, since we use
// different sources of truth depending on wether or not the instance is locally connected.
type InstanceStateInfo struct {
	LastSeen     time.Time
	Current      vc.Target
	StaticLabels map[string]string
	Log          InstanceControlLogScanner
}

func (c *Controller) getStatus(active *types.VersionDirectiveV1, info InstanceStateInfo) {
	now := c.cfg.Clock.Now()

	if now.After(info.LastSeen.Add(c.cfg.OfflineThreshold)) {
		// instance appears offline. If the instance went offline around the
		// time of a recent install attempt, this might be a churn event.
		if info.Log.InstallAttempted() && info.Log.InstallTime().Add(c.cfg.ChurnThreshold).After(info.LastSeen) {
			return InstanceStatusSummary{
				Status:    StatusChurnedDuringInstall,
				Target:    info.Log.Target(),
				Installer: info.Log.Installer(),
				Error:     info.Log.Error(), // include explicit fault error if one exists
			}
		}

		// instance is just offline. does not connote any meaninful version control status.
		return InstanceStatusSummary{}
	}

	if info.Log.InstallFault() && info.Log.FaultTime().Add(c.cfg.FaultBackoffPeriod).After(now) {
		// instance hit an explicit fault and we are still in fault backoff period
		return InstanceStatusSummary{
			Status:    StatusExplicitInstallFault,
			Target:    info.Log.Target(),
			Installer: info.Log.Installer(),
			Error:     info.Log.Error(), // include explicit fault error if one exists
		}
	}

	if info.Log.InstallAttempted() {

		if info.Log.InstallTime().Add(c.cfg.InstallGracePeriod).After(now) {
			// install attempt is within grace period and has not experienced
			// an explicit fault (caught by previous case).
			if info.Current.VersionEquals(info.Log.Target()) {
				return InstanceStatusSummary{
					Status:    StatusRecentInstall,
					Target:    info.Log.Target(),
					Installer: info.Log.Installer(),
				}
			}

			return InstanceStatusSummary{
				Status:    StatusInstallTriggered,
				Target:    info.Log.Target(),
				Installer: info.Log.Installer(),
			}
		}

		if !info.Current.VersionEquals(info.Log.Target()) && info.Log.InstallTime().Add(c.cfg.FaultBackoffPeriod).After(now) {
			// current version does not match target version, and we are still within the fault backoff period.
			return InstanceStatusSummary{
				Status:    StatusImplicitInstallFault,
				Target:    info.Log.Target(),
				Installer: info.Log.Installer(),
			}
		}

		// install attempt old enough to avoid any backoff conditions
	}

	if active == nil {
		// no active directive: instance has no meaningful version control state.
		return InstanceStatusSummary{}
	}

	stanza := getDirectiveStanza(active, info)
	if stanza == nil {
		// no matching stanza: instance has no meaningful version control state.
		return InstanceStatusSummary{}
	}

	// check if stanza specifies a valid target for this instance
	var stanzaInstallTarget vc.Target
	for _, target := range stanza.Targets {
		// future version will need to select from the available targets based on
		// additional attributes (e.g. arch). for now, targets with any attributes
		// other than version are ignored.
		if target.Ok() && len(target) == 1 {
			stanzaInstallTarget = target
			break
		}
	}

	if !stanzaInstallTarget.Ok() {
		// no valid install targets: instance has no meaningful version control state.
		return InstanceStatusSummary{}
	}

	if stanzaInstallTarget.VersionEquals(info.Current) {
		// target is already at expected version
		return InstanceStatusSummary{
			Status: StatusVersionParity,
		}
	}

	var installer types.VersionControlInstallerRef
Outer:
	for _, preference := range info.Installers {
		for _, ref := range stanza.Installers {


		}
	}

	return InstanceStatusSummary{
		Status: StatusNeedsInstall,
		Target: stanzaInstallTarget,
		Installer: ,
	}

	if stanzaInstallTarget.Ok() {
		if stanzaInstallTarget.Version() == handle.Hello().Version {
			return InstanceStatusSummary{
				Status: StatusVersionParity,
			}
		}

		if !hasRecentInstall {
			return InstanceStatusSummary{
				Status: StatusNeedsInstall,
				Target: stanzaInstallTarget,
			}
		}

		if seq.Attempt.Labels[LogLabelTargetVersion] == stanzaInstallTarget.Version() {
			if seq.Attempt.Time.Add(c.installGracePeriod).After(time.Now()) {
				return InstanceStatusSummary{
					Status: StatusInstalling,
					Target: stanzaInstallTarget,
				}
			}
		}
	}
}

func getDirectiveStanza(active *types.VersionDirectiveV1, info InstanceStateInfo) *types.VersionDirectiveStanza {
	for idx := range active.Spec.Directives {
	Selectors:
		for _, selector := range active.Spec.Directives[idx].Selectors {
			if len(selector.Labels) == 0 {
				// a selector with no labels matches nothing
				continue Selectors
			}

			for key, val := range selector.Labels {
				if key == types.Wildcard && val == types.Wildcard {
					// special case: match everything
					continue
				}

				if ok, v := info.Labels[key]; !ok || (val != v && val != types.Wildcard) {
					// instance missing expected label
					// TODO(fspmarshall): match instance command labels.
					continue Selectors
				}
			}

			for _, service := range info.Services {
				// selector must match all services exposed by instance
				if !selector.MatchesService(service) {
					continue Selectors
				}
			}

			if len(selelctor.Current) > 0 {
				if !selector.Current.Ok() {
					// TODO(fspmarshall): add support for version matcher syntax
					continue Selectors
				}

				for key, val := range selector.Current {
					if info.Current[key] != val {
						continue Selectors
					}
				}
			}

			// if we got here, then the selector matches
			return &active.Spec.Directives[idx]
		}
	}

	return nil
}

// ControlLogScanner is a helper for gathering info from an instance's control log. This type
// uses a visitor-esque pattern, but requires two separate phases. First Scan is called on all
// items to determine the ID of the most recent install sequence, then, if one is discovered,
// Aggregate must be called to actually gather relevant info about the install sequence.
type ControlLogScanner struct {
	Attempt    types.InstanceControlLogEntry
	FirstFault types.InstanceControlLogEntry
}

// Scan looks for the ID of the most recent sequence of install events. Must be called
// for each control log element.
func (l *ControlLogScanner) Scan(entry types.InstanceControlLogEntry) {
	if entry.Type != LogEntryLocalInstallAttempt {
		return
	}

	if l.Attempt.Time.IsZero() || entry.Time.IsAfter(l.Attempt.Time) {
		l.Attempt = entry
	}
}

// InstallAttempted checks if this aggregator discovered an install attempt. If no install
// attempt was discovered, then the Aggregate phase can be skipped.
func (l *ControlLogScanner) InstallAttempted() bool {
	return !l.Attempt.Time.IsZero()
}

// InstallTime gets the time at which the install attempt was made.
func (l *ControlLogScanner) InstallTime() time.Time {
	return l.Attempt.Time
}

// Aggregate aggregates information about the install attempt discovered during the Scan
// phase. Must not be called until after Scan has been called on all entries, and should
// not be called at all if HasInstall
func (l *ControlLogScanner) Aggregate(entry types.InstanceControlLogEntry) {
	if !a.InstallAttempted() {
		return
	}

	if entry.ID != l.Attempt.ID || entry.Type != LogEntryLocalInstallFault {
		return
	}

	if l.FirstFault.Time.IsZero() || l.FirstFault.Time.IsAfter(entry.Time) {
		l.FirstFault = entry
	}
}

// TargetVersion gets the target of the install attempt (nil if no install attempt exists).
func (l *ControlLogScanner) Target() vc.Target {
	panic("TODO")
	//return l.Attempt.Target
}

// Installer gets the installer reference associated with the install attempt. If no installer
// ref is present, the returned value will have InstallerKindNone. Note that the values here are
// not validated in any way. We want to be able to display this installer ref even if we don't
// understand it.
func (l *ControlLogScanner) Installer() types.VersionControlInstallerRef {
	return types.VersionControlInstallerRef{
		Kind: types.VersionControlInstallerKind(l.Attempt.Labels[LogLabelInstallerKind]),
		Name: l.Attempt.Labels[LogLabelInstallerName],
	}
}

// InstallFault checks if this scanner has observed an install fault.
func (l *ControlLogScanner) InstallFault() bool {
	return !l.FirstFault.Time.IsZero()
}

func (l *ControlLogScanner) InstallFaultTime() time.Time {
	return l.FisrtFault.Time
}

// ErrorMessage loads the error message from the first fault, if one exists.
func (l *ControLogScanner) Error() string {
	return l.FirstFault.Labels[LogLabelError]
}

// scanControlLog scans an entire control log as a slice.
func scanControlLog(log []types.InstanceControlLogEntry) (scanner ControlLogScanner) {
	for _, entry := range log {
		scanner.Scan(entry)
	}

	if !scanner.InstallAttempted() {
		return
	}

	for _, entry := range log {
		scanner.Aggregate(entry)
	}
}

// scanRefControlLog scans the control log entries associated with an InstanceStateRef.
func scanRefControlLog(ref inventory.InstanceStateRef) (scanner ControlLogScanner) {
	// find the most recent install attempt entry
	ref.IterLogEntries(func(entry types.InstanceControlLogEntry) {
		scanner.Scan(entry)
	})

	if !scanner.InstallAttempted() {
		return
	}

	// find the oldest install fault entry associated with the latest attempt
	ref.IterLogEntries(func(entry types.VersionControlLogEntry) {
		scanner.Aggregate(entry)
	})
}
