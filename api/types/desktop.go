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

package types

import (
	"github.com/gravitational/trace"
)

// WindowsDesktopService represents a Windows desktop service instance.
type WindowsDesktopService interface {
	// Resource provides common resource methods.
	Resource
	// GetAddr returns the network address of this service.
	GetAddr() string
	// GetVersion returns the teleport binary version of this service.
	GetTeleportVersion() string
}

var _ WindowsDesktopService = &WindowsDesktopServiceV3{}

// NewWindowsDesktopServiceV3 creates a new WindowsDesktopServiceV3 resource.
func NewWindowsDesktopServiceV3(name string, spec WindowsDesktopServiceSpecV3) (*WindowsDesktopServiceV3, error) {
	s := &WindowsDesktopServiceV3{
		ResourceHeader: ResourceHeader{
			Metadata: Metadata{
				Name: name,
			},
		},
		Spec: spec,
	}
	if err := s.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return s, nil
}

func (s *WindowsDesktopServiceV3) setStaticFields() {
	s.Kind = KindWindowsDesktopService
	s.Version = V3
}

// CheckAndSetDefaults checks and sets default values for any missing fields.
func (s *WindowsDesktopServiceV3) CheckAndSetDefaults() error {
	if s.Spec.Addr == "" {
		return trace.BadParameter("WindowsDesktopServiceV3.Spec missing Addr field")
	}
	if s.Spec.TeleportVersion == "" {
		return trace.BadParameter("WindowsDesktopServiceV3.Spec missing TeleportVersion field")
	}

	s.setStaticFields()
	if err := s.ResourceHeader.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetAddr returns the network address of this service.
func (s *WindowsDesktopServiceV3) GetAddr() string {
	return s.Spec.Addr
}

// GetTeleportVersion returns the teleport binary version of this service.
func (s *WindowsDesktopServiceV3) GetTeleportVersion() string {
	return s.Spec.TeleportVersion
}

// WindowsDesktop represents a Windows desktop host.
type WindowsDesktop interface {
	// Resource provides common resource methods.
	Resource
	// GetAddr returns the network address of this host.
	GetAddr() string
	// GetAllLabels returns combined static and dynamic labels.
	GetAllLabels() map[string]string
	// LabelsString returns all labels as a string.
	LabelsString() string
	// GetDomain returns the ActiveDirectory domain of this host.
	GetDomain() string
	// GetHostID returns the ID of the Windows Desktop Service reporting the desktop.
	GetHostID() string
}

var _ WindowsDesktop = &WindowsDesktopV3{}

// NewWindowsDesktopV3 creates a new WindowsDesktopV3 resource.
func NewWindowsDesktopV3(name string, labels map[string]string, spec WindowsDesktopSpecV3) (*WindowsDesktopV3, error) {
	d := &WindowsDesktopV3{
		ResourceHeader: ResourceHeader{
			Metadata: Metadata{
				Name:   name,
				Labels: labels,
			},
		},
		Spec: spec,
	}
	if err := d.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return d, nil
}

func (d *WindowsDesktopV3) setStaticFields() {
	d.Kind = KindWindowsDesktop
	d.Version = V3
}

// CheckAndSetDefaults checks and sets default values for any missing fields.
func (d *WindowsDesktopV3) CheckAndSetDefaults() error {
	if d.Spec.Addr == "" {
		return trace.BadParameter("WindowsDesktopV3.Spec missing Addr field")
	}

	d.setStaticFields()
	if err := d.ResourceHeader.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetAddr returns the network address of this host.
func (d *WindowsDesktopV3) GetAddr() string {
	return d.Spec.Addr
}

// GetHostID returns the HostID for the associated desktop service.
func (d *WindowsDesktopV3) GetHostID() string {
	return d.Spec.HostID
}

// GetAllLabels returns combined static and dynamic labels.
func (d *WindowsDesktopV3) GetAllLabels() map[string]string {
	// TODO(zmb3): add dynamic labels when running in agent mode
	return CombineLabels(d.Metadata.Labels, nil)
}

// LabelsString returns all desktop labels as a string.
func (d *WindowsDesktopV3) LabelsString() string {
	return LabelsAsString(d.Metadata.Labels, nil)
}

// GetDomain returns the Active Directory domain of this host.
func (d *WindowsDesktopV3) GetDomain() string {
	return d.Spec.Domain
}

// Origin returns the origin value of the resource.
func (d *WindowsDesktopV3) Origin() string {
	return d.Metadata.Labels[OriginLabel]
}

// SetOrigin sets the origin value of the resource.
func (d *WindowsDesktopV3) SetOrigin(o string) {
	d.Metadata.Labels[OriginLabel] = o
}

// DeduplicateDesktops deduplicates desktops by name.
func DeduplicateDesktops(desktops []WindowsDesktop) (result []WindowsDesktop) {
	seen := make(map[string]struct{})
	for _, desktop := range desktops {
		if _, ok := seen[desktop.GetName()]; ok {
			continue
		}
		seen[desktop.GetName()] = struct{}{}
		result = append(result, desktop)
	}
	return result
}

// Match checks if a given desktop request matches this filter.
func (f *WindowsDesktopFilter) Match(req WindowsDesktop) bool {
	if f.HostID != "" && req.GetHostID() != f.HostID {
		return false
	}
	if f.Name != "" && req.GetName() != f.Name {
		return false
	}
	return true
}
