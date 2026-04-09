/*
Copyright 2023 Gravitational, Inc.

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

package mfa

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
)

// Prompt is an MFA prompt.
type Prompt interface {
	// Run prompts the user to complete an MFA authentication challenge.
	Run(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error)
	// AskRegister prompts the user for device details for a new MFA device.
	AskRegister(ctx context.Context, config RegistrationPromptConfig) (RegisterConfig, error)
	// RunRegister prompts the user to complete a registration challenge.
	RunRegister(ctx context.Context, config RegisterConfig, chal *proto.MFARegisterChallenge) (*RegistrationResult, error)
	// NotifyRegistrationSuccess notifies the user that the device registration
	// was successful.
	NotifyRegistrationSuccess(ctx context.Context, cfg RegisterConfig) error
}

// PromptFunc is a function wrapper that implements the Prompt interface.
type PromptFunc func(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error)

// Run prompts the user to complete an MFA authentication challenge.
func (f PromptFunc) Run(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
	return f(ctx, chal)
}

// AskRegister prompts the user for device details for a new MFA device.
func (f PromptFunc) AskRegister(ctx context.Context, config RegistrationPromptConfig) (RegisterConfig, error) {
	return RegisterConfig{}, trace.NotImplemented("not supported")
}

// RunRegister prompts the user to complete a registration challenge.
func (f PromptFunc) RunRegister(ctx context.Context, config RegisterConfig, chal *proto.MFARegisterChallenge) (*RegistrationResult, error) {
	return nil, trace.NotImplemented("not supported")
}

// NotifyRegistrationSuccess notifies the user that the device registration was
// successful.
func (f PromptFunc) NotifyRegistrationSuccess(ctx context.Context, config RegisterConfig) error {
	return trace.NotImplemented("not supported")
}

// PromptConstructor is a function that creates a new MFA prompt.
type PromptConstructor func(...PromptOpt) Prompt

// PromptConfig contains universal mfa prompt config options.
type PromptConfig struct {
	// PromptReason is an optional message to share with the user before an MFA Prompt.
	// It is intended to provide context about why the user is being prompted where it may
	// not be obvious, such as for admin actions or per-session MFA.
	PromptReason string
	// DeviceType is an optional device description to emphasize during the prompt.
	DeviceType DeviceDescriptor
	// Quiet suppresses users prompts.
	Quiet bool
	// PerSessionMFA indicates that the prompt is being used for a per-session
	// MFA ceremony. It is used only for prompt presentation and local UX.
	PerSessionMFA bool
	// CallbackCeremony is an SSO or Browser MFA ceremony.
	CallbackCeremony CallbackCeremony
}

// RegistrationResult contains the result of a [Prompt.RunRegister] call.
type RegistrationResult struct {
	// Response is the registration challenge response from the MFA device.
	Response *proto.MFARegisterResponse
	// Callbacks contain functions that need to be called depending on the result
	// of adding the MFA device to the Teleport backend. They may have no effect,
	// depending if they are supported by the particular MFA technology.
	Callbacks RegistrationCallbacks
}

// RegistrationCallbacks contains functions for confirming or rolling back
// credentials that have been created by the MFA device.
type RegistrationCallbacks interface {
	// Rollback removes the newly created key from the MFA device.
	Rollback() error
	// Confirm persists the newly created key in the MFA device.
	Confirm() error
}

// RegistrationPromptConfig provides configuration for the [Prompt.AskRegister]
// function.
type RegistrationPromptConfig struct {
	RegisterConfig

	// Reason indicates the reason why we register an MFA device.
	Reason RegistrationReason
	// DeviceTypeOptions are the options for the type of the device to be added.
	// If there are multiple options, the user be prompted to pick one.
	DeviceTypeOptions []MFADeviceType
}

// RegisterConfig
type RegisterConfig struct {
	// DeviceName is the name of the device to be added. If empty, the user will
	// be prompted to enter it.
	DeviceName string
	// DeviceType
	DeviceType MFADeviceType
	// DeviceUsage is the intended usage for the MFA device to be added. If set
	// to [proto.DeviceUsage_DEVICE_USAGE_UNSPECIFIED], the user may be offered
	// registering a passwordless device.
	DeviceUsage proto.DeviceUsage
}

// PromptOpt applies configuration options to a prompt.
type PromptOpt func(*PromptConfig)

// WithQuiet sets the prompt's Quiet field.
func WithQuiet() PromptOpt {
	return func(cfg *PromptConfig) {
		cfg.Quiet = true
	}
}

// WithPromptReason sets the prompt's PromptReason field.
func WithPromptReason(hint string) PromptOpt {
	return func(cfg *PromptConfig) {
		cfg.PromptReason = hint
	}
}

// WithPromptReasonAdminAction sets the prompt's PromptReason field to a standard admin action message.
func WithPromptReasonAdminAction() PromptOpt {
	const adminMFAPromptReason = "This is an admin-level action and requires MFA to complete"
	return WithPromptReason(adminMFAPromptReason)
}

// WithPromptReasonSessionMFA sets the prompt's PromptReason field to a standard session mfa message.
func WithPromptReasonSessionMFA(serviceType, serviceName string) PromptOpt {
	return func(cfg *PromptConfig) {
		cfg.PromptReason = fmt.Sprintf("MFA is required to access %s %q", serviceType, serviceName)
		cfg.PerSessionMFA = true
	}
}

// DeviceDescriptor is a descriptor for a device, such as "registered".
type DeviceDescriptor string

const (
	// DeviceDescriptorRegistered is a registered device.
	DeviceDescriptorRegistered = "registered"
	// DeviceDescriptorNew is a new device being registered.
	DeviceDescriptorNew = "new"
)

// WithPromptDeviceType sets the prompt's DeviceType field.
func WithPromptDeviceType(deviceType DeviceDescriptor) PromptOpt {
	return func(cfg *PromptConfig) {
		cfg.DeviceType = deviceType
	}
}

// withSSOMFACeremony sets the SSO MFA ceremony for the MFA prompt.
func withSSOMFACeremony(ssoMFACeremony CallbackCeremony) PromptOpt {
	return func(cfg *PromptConfig) {
		cfg.CallbackCeremony = ssoMFACeremony
	}
}
