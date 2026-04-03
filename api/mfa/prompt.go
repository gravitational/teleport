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
	"github.com/gravitational/teleport/api/constants"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
)

// MFADeviceType is a type of MFA device for device registration purposes.
type MFADeviceType string

const (
	TOTPDeviceType     MFADeviceType = "TOTP"
	WebauthnDeviceType MFADeviceType = "WEBAUTHN"
	TouchIDDeviceType  MFADeviceType = "TOUCHID"
)

// RegistrationCallbacks contains functions for confirming or rolling back
// credentials that have been created by the MFA device.
type RegistrationCallbacks interface {
	// Rollback removes the newly created key from the MFA device.
	Rollback() error
	// Confirm persists the newly created key in the MFA device.
	Confirm() error
}

// RegistrationResult contains the result of a [Prompt.AskRegister] call.
type RegistrationResult struct {
	// Config is the registration ceremony config, potentially updated with user
	// input (device name, type, and usage) if not provided upfront.
	Config RegistrationPromptConfig
	// Response is the registration challenge response from the MFA device.
	Response *proto.MFARegisterResponse
	// Callbacks contain functions that need to be called depending on the result
	// of adding the MFA device to the Teleport backend. They may have no effect,
	// depending if they are supported by the particular MFA technology.
	Callbacks RegistrationCallbacks
}

// RegistrationPromptConfig provides configuration for the [Prompt.AskRegister]
// function.
type RegistrationPromptConfig struct {
	RegistrationCeremonyConfig

	// AuthSecondFactor is the Second Factor option as configured in the auth
	// service's auth preferences.
	AuthSecondFactor constants.SecondFactorType
}

// Prompt is an MFA prompt.
type Prompt interface {
	// Run prompts the user to complete an MFA authentication challenge.
	Run(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error)
	// AskRegister prompts user for device details and registers a new MFA
	// device.
	AskRegister(ctx context.Context, config RegistrationPromptConfig) (*RegistrationResult, error)
	// NotifyRegistrationSuccess notifies the user that the device registration
	// was successful.
	NotifyRegistrationSuccess(ctx context.Context, config RegistrationPromptConfig) error
}

// PromptFunc is a function wrapper that implements the Prompt interface.
type PromptFunc func(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error)

// Run prompts the user to complete an MFA authentication challenge.
func (f PromptFunc) Run(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
	return f(ctx, chal)
}

// AskRegister prompts user for device details and registers a new MFA device.
func (f PromptFunc) AskRegister(ctx context.Context, config RegistrationPromptConfig) (*RegistrationResult, error) {
	return nil, trace.NotImplemented("not supported")
}

// NotifyRegistrationSuccess notifies the user that the device registration was
// successful.
func (f PromptFunc) NotifyRegistrationSuccess(ctx context.Context, config RegistrationPromptConfig) error {
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
	// Extensions are the challenge extensions used to create the prompt's challenge.
	// Used to enrich certain prompts.
	Extensions *mfav1.ChallengeExtensions
	// CallbackCeremony is an SSO or Browser MFA ceremony.
	CallbackCeremony CallbackCeremony
	// MFACeremony is an MFA ceremony. Used when the prompt is used for
	// registering a new MFA device.
	MFACeremony *Ceremony
}

// DeviceDescriptor is a descriptor for a device, such as "registered".
type DeviceDescriptor string

// DeviceDescriptorRegistered is a registered device.
const DeviceDescriptorRegistered = "registered"

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

		// Set the extensions to scope USER_SESSION, which we know is true, but
		// don't override any explicitly-set extensions (as they are likely more
		// complete).
		if cfg.Extensions == nil {
			cfg.Extensions = &mfav1.ChallengeExtensions{
				Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_USER_SESSION,
			}
		}
	}
}

// WithPromptDeviceType sets the prompt's DeviceType field.
func WithPromptDeviceType(deviceType DeviceDescriptor) PromptOpt {
	return func(cfg *PromptConfig) {
		cfg.DeviceType = deviceType
	}
}

// WithPromptChallengeExtensions sets the challenge extensions used to create
// the prompt's challenge.
// While not mandatory, informing the prompt of the extensions used allows for
// better user messaging.
func WithPromptChallengeExtensions(exts *mfav1.ChallengeExtensions) PromptOpt {
	return func(cfg *PromptConfig) {
		cfg.Extensions = exts
	}
}

// withSSOMFACeremony sets the SSO MFA ceremony for the MFA prompt.
func withSSOMFACeremony(ssoMFACeremony CallbackCeremony) PromptOpt {
	return func(cfg *PromptConfig) {
		cfg.CallbackCeremony = ssoMFACeremony
	}
}
