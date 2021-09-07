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
	// GetAddr returns the network address of this service.
	GetAddr() string
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
