/*
Copyright 2024 Gravitational, Inc.

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

package provider

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"strings"
	"text/template"
	"time"

	"github.com/gravitational/trace"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/constants"
	apitypes "github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/lib/embeddedtbot"
	tbotconfig "github.com/gravitational/teleport/lib/tbot/config"
)

var supportedCredentialSources = CredentialSources{
	CredentialsFromNativeMachineID{},
	CredentialsFromKeyAndCertPath{},
	CredentialsFromKeyAndCertBase64{},
	CredentialsFromIdentityFilePath{},
	CredentialsFromIdentityFileString{},
	CredentialsFromIdentityFileBase64{},
	CredentialsFromProfile{},
}

// CredentialSources is a list of CredentialSource
type CredentialSources []CredentialSource

// ActiveSources returns the list of active sources, and an error diagnostic if no source is active.
// The error diagnostic explains why every source is inactive.
func (s CredentialSources) ActiveSources(ctx context.Context, config providerData) (CredentialSources, diag.Diagnostics) {
	var activeSources CredentialSources
	inactiveReason := strings.Builder{}
	for _, source := range s {
		active, reason := source.IsActive(config)
		logFields := map[string]interface{}{
			"source": source.Name(),
			"active": active,
			"reason": reason,
		}
		if !active {
			tflog.Info(ctx, "credentials source is not active, skipping", logFields)
			inactiveReason.WriteString(fmt.Sprintf(" - cannot read credentials %s because %s\n", source.Name(), reason))
			continue
		}
		tflog.Info(ctx, "credentials source is active", logFields)
		activeSources = append(activeSources, source)
	}
	if len(activeSources) == 0 {
		// TODO: make this a hard failure in v17
		// We currently try to load credentials from the user profile.
		// As trying broken credentials takes 30 seconds this is a very bad UX and we should get rid of this.
		// Credentials from profile are not passing MFA4Admin anyway.
		summary := inactiveReason.String() +
			"\nThe provider will fallback to your current local profile (this behavior is deprecated and will be removed in v17, you should specify the profile name or directory)."
		return CredentialSources{CredentialsFromProfile{isDefault: true}}, diag.Diagnostics{diag.NewWarningDiagnostic(
			"No active Teleport credentials source found",
			summary,
		)}
	}
	return activeSources, nil
}

// BuildClient sequentially builds credentials for every source and tries to use them to connect to Teleport.
// Any CredentialSource failing to return a Credential and a tls.Config causes a hard failure.
// If we have a valid credential but cannot connect, we send a warning and continue with the next credential
// (this is for backward compatibility).
// Expired credentials are skipped for the sake of UX. This is the most common failure mode and we can
// return an error quickly instead of hanging for 30 whole seconds.
func (s CredentialSources) BuildClient(ctx context.Context, clientCfg client.Config, providerCfg providerData) (*client.Client, diag.Diagnostics) {
	diags := diag.Diagnostics{}
	for _, source := range s {
		logFields := map[string]interface{}{
			"source": source.Name(),
		}
		tflog.Info(ctx, fmt.Sprintf("trying to build a client %s", source.Name()), logFields)
		creds, err := source.Credentials(ctx, providerCfg)
		if err != nil {
			logFields["error"] = err.Error()
			tflog.Error(ctx, "failed to obtain credential", logFields)
			_, reason := source.IsActive(providerCfg)
			diags.AddError(
				fmt.Sprintf("Failed to obtain Teleport credentials %s", source.Name()),
				brokenCredentialErrorSummary(source.Name(), reason, err),
			)
			return nil, diags
		}

		// Smoke test to see if the credential is valid
		// This catches all the "file not found" issues and other broken credentials
		// so we can turn them into a hard failure.
		_, err = creds.TLSConfig()
		if err != nil {
			logFields["error"] = err.Error()
			tflog.Error(ctx, "failed to get a TLSConfig from the credential", logFields)
			_, reason := source.IsActive(providerCfg)
			diags.AddError(
				fmt.Sprintf("Invalid Teleport credentials %s", source.Name()),
				brokenCredentialErrorSummary(source.Name(), reason, err),
			)

			return nil, diags
		}

		now := time.Now()
		if expiry, ok := creds.Expiry(); ok && !expiry.IsZero() && expiry.Before(now) {
			diags.AddWarning(
				fmt.Sprintf("Teleport credentials %s are expired", source.Name()),
				fmt.Sprintf(`The credentials %s are expired. Expiration is %q while current time is %q). You might need to refresh them. The provider will not attempt to use those credentials.`,
					source.Name(), expiry.Local(), now.Local()),
			)
			continue
		}

		clientCfg.Credentials = []client.Credentials{creds}
		// In case of connection failure, this takes 30 seconds to return, which is very, very long.
		clt, err := client.New(ctx, clientCfg)
		if err != nil {
			logFields["error"] = err.Error()
			tflog.Error(ctx, "failed to connect with the credential", logFields)
			diags.AddWarning(
				fmt.Sprintf("Failed to connect with credentials %s", source.Name()),
				fmt.Sprintf("The client built from the credentials %s failed to connect to %q with the error: %s.",
					source.Name(), clientCfg.Addrs[0], err,
				))
			continue
		}
		// A client was successfully built
		return clt, diags
	}
	// No client was built
	diags.AddError("Impossible to build Teleport client", s.failedToBuildClientErrorSummary(clientCfg.Addrs[0]))
	return nil, diags
}

const failedToBuildClientErrorTemplate = `"Every credential source provided has failed. The Terraform provider cannot connect to the Teleport cluster '{{.Addr}}'.

The provider tried building a client:
{{- range $_, $source := .Sources }}
- {{ $source }}
{{- end }}

You can find more information about why each credential source failed in the Terraform warnings above this error.`

// failedToBuildClientErrorSummary builds a user-friendly message explaining we failed to build a functional Teleport
// client and listing every connection method we tried.
func (s CredentialSources) failedToBuildClientErrorSummary(addr string) string {
	var sources []string
	for _, source := range s {
		sources = append(sources, source.Name())
	}

	tpl := template.Must(template.New("failed-to-build-client-error-summary").Parse(failedToBuildClientErrorTemplate))
	values := struct {
		Addr    string
		Sources []string
	}{
		Addr:    addr,
		Sources: sources,
	}
	buffer := new(bytes.Buffer)
	err := tpl.Execute(buffer, values)
	if err != nil {
		return "Failed to build error summary. This is a provider bug: " + err.Error()
	}
	return buffer.String()
}

const brokenCredentialErrorTemplate = `The Terraform provider tried to build credentials {{ .Source }} but received the following error:

{{ .Error }}

The provider tried to use the credential source because {{ .Reason }}. You must either address the error or disable the credential source by removing its values.`

// brokenCredentialErrorSummary returns a user-friendly message explaining why we failed to
func brokenCredentialErrorSummary(name, activeReason string, err error) string {
	tpl := template.Must(template.New("broken-credential-error-summary").Parse(brokenCredentialErrorTemplate))
	values := struct {
		Source string
		Error  string
		Reason string
	}{
		Source: name,
		Error:  err.Error(),
		Reason: activeReason,
	}
	buffer := new(bytes.Buffer)
	tplErr := tpl.Execute(buffer, values)
	if tplErr != nil {
		return fmt.Sprintf("Failed to build error '%s' summary. This is a provider bug: %s", err, tplErr)
	}
	return buffer.String()
}

// CredentialSource is a potential way for the Terraform provider to obtain the
// client.Credentials needed to connect to the Teleport cluster.
// A CredentialSource is active if the user specified configuration specific to this source.
// Only active CredentialSources are considered by the Provider.
type CredentialSource interface {
	Name() string
	IsActive(providerData) (bool, string)
	Credentials(context.Context, providerData) (client.Credentials, error)
}

// CredentialsFromKeyAndCertPath builds credentials from key, cert and ca cert paths.
type CredentialsFromKeyAndCertPath struct{}

// Name implements CredentialSource and returns the source name.
func (CredentialsFromKeyAndCertPath) Name() string {
	return "from Key, Cert, and CA path"
}

// IsActive implements CredentialSource and returns if the source is active and why.
func (CredentialsFromKeyAndCertPath) IsActive(config providerData) (bool, string) {
	certPath := stringFromConfigOrEnv(config.CertPath, constants.EnvVarTerraformCertificates, "")
	keyPath := stringFromConfigOrEnv(config.KeyPath, constants.EnvVarTerraformKey, "")

	// This method is active as soon as a cert or a key path are set.
	active := certPath != "" || keyPath != ""

	return activeReason(
		active,
		attributeTerraformCertificates, attributeTerraformKey,
		constants.EnvVarTerraformCertificates, constants.EnvVarTerraformKey,
	)
}

// Credentials implements CredentialSource and returns a client.Credentials for the provider.
func (CredentialsFromKeyAndCertPath) Credentials(ctx context.Context, config providerData) (client.Credentials, error) {
	certPath := stringFromConfigOrEnv(config.CertPath, constants.EnvVarTerraformCertificates, "")
	keyPath := stringFromConfigOrEnv(config.KeyPath, constants.EnvVarTerraformKey, "")
	caPath := stringFromConfigOrEnv(config.RootCaPath, constants.EnvVarTerraformRootCertificates, "")

	// Validate that we have all paths.
	if certPath == "" {
		return nil, trace.BadParameter("missing parameter %q or environment variable %q", attributeTerraformCertificates, constants.EnvVarTerraformCertificates)
	}
	if keyPath == "" {
		return nil, trace.BadParameter("missing parameter %q or environment variable %q", attributeTerraformKey, constants.EnvVarTerraformKey)
	}
	if caPath == "" {
		return nil, trace.BadParameter("missing parameter %q or environment variable %q", attributeTerraformRootCertificates, constants.EnvVarTerraformRootCertificates)
	}

	// Validate the files exist for a better UX?

	creds := client.LoadKeyPair(certPath, keyPath, caPath)
	return creds, nil
}

// CredentialsFromKeyAndCertBase64 builds credentials from key, cert, and CA cert base64.
type CredentialsFromKeyAndCertBase64 struct{}

// Name implements CredentialSource and returns the source name.
func (CredentialsFromKeyAndCertBase64) Name() string {
	return "from Key, Cert, and CA base64"
}

// IsActive implements CredentialSource and returns if the source is active and why.
func (CredentialsFromKeyAndCertBase64) IsActive(config providerData) (bool, string) {
	certBase64 := stringFromConfigOrEnv(config.CertBase64, constants.EnvVarTerraformCertificatesBase64, "")
	keyBase64 := stringFromConfigOrEnv(config.KeyBase64, constants.EnvVarTerraformKeyBase64, "")

	// This method is active as soon as a cert or a key is passed.
	active := certBase64 != "" || keyBase64 != ""

	return activeReason(
		active,
		attributeTerraformCertificatesBase64, attributeTerraformKeyBase64,
		constants.EnvVarTerraformCertificatesBase64, constants.EnvVarTerraformKeyBase64,
	)
}

// Credentials implements CredentialSource and returns a client.Credentials for the provider.
func (CredentialsFromKeyAndCertBase64) Credentials(ctx context.Context, config providerData) (client.Credentials, error) {
	certBase64 := stringFromConfigOrEnv(config.CertBase64, constants.EnvVarTerraformCertificatesBase64, "")
	keyBase64 := stringFromConfigOrEnv(config.KeyBase64, constants.EnvVarTerraformKeyBase64, "")
	caBase64 := stringFromConfigOrEnv(config.RootCaBase64, constants.EnvVarTerraformRootCertificatesBase64, "")

	// Validate that we have all paths.
	if certBase64 == "" {
		return nil, trace.BadParameter("missing parameter %q or environment variable %q", attributeTerraformCertificatesBase64, constants.EnvVarTerraformCertificatesBase64)
	}
	if keyBase64 == "" {
		return nil, trace.BadParameter("missing parameter %q or environment variable %q", attributeTerraformKeyBase64, constants.EnvVarTerraformKeyBase64)
	}
	if caBase64 == "" {
		return nil, trace.BadParameter("missing parameter %q or environment variable %q", attributeTerraformRootCertificatesBase64, constants.EnvVarTerraformRootCertificatesBase64)
	}

	certPEM, err := base64.StdEncoding.DecodeString(certBase64)
	if err != nil {
		return nil, trace.Wrap(err, "failed to decode the certificate's base64 (standard b64 encoding)")
	}
	keyPEM, err := base64.StdEncoding.DecodeString(keyBase64)
	if err != nil {
		return nil, trace.Wrap(err, "failed to decode the key's base64 (standard b64 encoding)")
	}
	caPEM, err := base64.StdEncoding.DecodeString(caBase64)
	if err != nil {
		return nil, trace.Wrap(err, "failed to decode the CA's base64 (standard b64 encoding)")
	}

	creds, err := client.KeyPair(certPEM, keyPEM, caPEM)
	return creds, trace.Wrap(err, "failed to load credentials from the PEM-encoded key and certificate")
}

// CredentialsFromIdentityFilePath builds credentials from an identity file path.
type CredentialsFromIdentityFilePath struct{}

// Name implements CredentialSource and returns the source name.
func (CredentialsFromIdentityFilePath) Name() string {
	return "from the identity file path"
}

// IsActive implements CredentialSource and returns if the source is active and why.
func (CredentialsFromIdentityFilePath) IsActive(config providerData) (bool, string) {
	identityFilePath := stringFromConfigOrEnv(config.IdentityFilePath, constants.EnvVarTerraformIdentityFilePath, "")

	active := identityFilePath != ""

	return activeReason(
		active,
		attributeTerraformIdentityFilePath, constants.EnvVarTerraformIdentityFilePath,
	)
}

// Credentials implements CredentialSource and returns a client.Credentials for the provider.
func (CredentialsFromIdentityFilePath) Credentials(ctx context.Context, config providerData) (client.Credentials, error) {
	identityFilePath := stringFromConfigOrEnv(config.IdentityFilePath, constants.EnvVarTerraformIdentityFilePath, "")

	return client.LoadIdentityFile(identityFilePath), nil
}

// CredentialsFromIdentityFileString builds credentials from an identity file passed as a string.
type CredentialsFromIdentityFileString struct{}

// Name implements CredentialSource and returns the source name.
func (CredentialsFromIdentityFileString) Name() string {
	return "from the identity file (passed as a string)"
}

// IsActive implements CredentialSource and returns if the source is active and why.
func (CredentialsFromIdentityFileString) IsActive(config providerData) (bool, string) {
	identityFileString := stringFromConfigOrEnv(config.IdentityFile, constants.EnvVarTerraformIdentityFile, "")

	active := identityFileString != ""

	return activeReason(
		active,
		attributeTerraformIdentityFile, constants.EnvVarTerraformIdentityFile,
	)
}

// Credentials implements CredentialSource and returns a client.Credentials for the provider.
func (CredentialsFromIdentityFileString) Credentials(ctx context.Context, config providerData) (client.Credentials, error) {
	identityFileString := stringFromConfigOrEnv(config.IdentityFile, constants.EnvVarTerraformIdentityFile, "")

	return client.LoadIdentityFileFromString(identityFileString), nil
}

// CredentialsFromIdentityFileBase64 builds credentials from an identity file passed as a base64-encoded string.
type CredentialsFromIdentityFileBase64 struct{}

// Name implements CredentialSource and returns the source name.
func (CredentialsFromIdentityFileBase64) Name() string {
	return "from the identity file (passed as a base64-encoded string)"
}

// IsActive implements CredentialSource and returns if the source is active and why.
func (CredentialsFromIdentityFileBase64) IsActive(config providerData) (bool, string) {
	identityFileBase64 := stringFromConfigOrEnv(config.IdentityFileBase64, constants.EnvVarTerraformIdentityFileBase64, "")

	// This method is active as soon as a cert or a key path are set.
	active := identityFileBase64 != ""

	return activeReason(
		active,
		attributeTerraformIdentityFileBase64, constants.EnvVarTerraformIdentityFileBase64,
	)
}

// Credentials implements CredentialSource and returns a client.Credentials for the provider.
func (CredentialsFromIdentityFileBase64) Credentials(ctx context.Context, config providerData) (client.Credentials, error) {
	identityFileBase64 := stringFromConfigOrEnv(config.IdentityFileBase64, constants.EnvVarTerraformIdentityFileBase64, "")

	identityFile, err := base64.StdEncoding.DecodeString(identityFileBase64)
	if err != nil {
		return nil, trace.Wrap(err, "decoding base64 identity file")
	}

	return client.LoadIdentityFileFromString(string(identityFile)), nil
}

// CredentialsFromProfile builds credentials from a local tsh profile.
type CredentialsFromProfile struct {
	// isDefault represent if the CredentialSource is used as the default one.
	// In this case, it explains that it is always active.
	isDefault bool
}

// Name implements CredentialSource and returns the source name.
func (c CredentialsFromProfile) Name() string {
	name := "from the local profile"
	if c.isDefault {
		name += " (default)"
	}
	return name
}

// IsActive implements CredentialSource and returns if the source is active and why.
func (c CredentialsFromProfile) IsActive(config providerData) (bool, string) {
	if c.isDefault {
		return true, "this is the default credential source, and no other credential was active"
	}

	profileName := stringFromConfigOrEnv(config.ProfileName, constants.EnvVarTerraformProfileName, "")
	profileDir := stringFromConfigOrEnv(config.ProfileDir, constants.EnvVarTerraformProfilePath, "")

	// This method is active as soon as a cert or a key path are set.
	active := profileDir != "" || profileName != ""
	return activeReason(
		active,
		attributeTerraformProfileName, attributeTerraformProfilePath,
		constants.EnvVarTerraformProfileName, constants.EnvVarTerraformProfilePath,
	)
}

// Credentials implements CredentialSource and returns a client.Credentials for the provider.
func (CredentialsFromProfile) Credentials(ctx context.Context, config providerData) (client.Credentials, error) {
	profileName := stringFromConfigOrEnv(config.ProfileName, constants.EnvVarTerraformProfileName, "")
	profileDir := stringFromConfigOrEnv(config.ProfileDir, constants.EnvVarTerraformProfilePath, "")

	return client.LoadProfile(profileDir, profileName), nil
}

// CredentialsFromNativeMachineID builds credentials by performing a MachineID join and
type CredentialsFromNativeMachineID struct{}

// Name implements CredentialSource and returns the source name.
func (CredentialsFromNativeMachineID) Name() string {
	return "by performing native MachineID joining"
}

// IsActive implements CredentialSource and returns if the source is active and why.
func (CredentialsFromNativeMachineID) IsActive(config providerData) (bool, string) {
	joinMethod := stringFromConfigOrEnv(config.JoinMethod, constants.EnvVarTerraformJoinMethod, "")
	joinToken := stringFromConfigOrEnv(config.JoinToken, constants.EnvVarTerraformJoinToken, "")

	// This method is active as soon as a token or a join method are set.
	active := joinMethod != "" || joinToken != ""
	return activeReason(
		active,
		attributeTerraformJoinMethod, attributeTerraformJoinToken,
		constants.EnvVarTerraformJoinMethod, constants.EnvVarTerraformJoinToken,
	)
}

// Credentials implements CredentialSource and returns a client.Credentials for the provider.
func (CredentialsFromNativeMachineID) Credentials(ctx context.Context, config providerData) (client.Credentials, error) {
	joinMethod := stringFromConfigOrEnv(config.JoinMethod, constants.EnvVarTerraformJoinMethod, "")
	joinToken := stringFromConfigOrEnv(config.JoinToken, constants.EnvVarTerraformJoinToken, "")
	audienceTag := stringFromConfigOrEnv(config.AudienceTag, constants.EnvVarTerraformCloudJoinAudienceTag, "")
	addr := stringFromConfigOrEnv(config.Addr, constants.EnvVarTerraformAddress, "")
	caPath := stringFromConfigOrEnv(config.RootCaPath, constants.EnvVarTerraformRootCertificates, "")

	if joinMethod == "" {
		return nil, trace.BadParameter("missing parameter %q or environment variable %q", attributeTerraformJoinMethod, constants.EnvVarTerraformJoinMethod)
	}
	if joinToken == "" {
		return nil, trace.BadParameter("missing parameter %q or environment variable %q", attributeTerraformJoinMethod, constants.EnvVarTerraformJoinMethod)
	}
	if addr == "" {
		return nil, trace.BadParameter("missing parameter %q or environment variable %q", attributeTerraformAddress, constants.EnvVarTerraformAddress)
	}

	if apitypes.JoinMethod(joinMethod) == apitypes.JoinMethodToken {
		return nil, trace.BadParameter(`the secret token join method ('token') is not supported for native Machine ID joining.

Secret tokens are single use and the Terraform provider does not save the certificates it obtained, so the token join method can only be used once.
If you want to run the Terraform provider in the CI (GitHub Actions, GitlabCI, Circle CI) or in a supported runtime (AWS, GCP, Azure, Kubernetes, machine with a TPM)
you should use the join method specific to your environment.
If you want to use MachineID with secret tokens, the best approach is to run a local tbot on the server where the terraform provider runs.

See https://goteleport.com/docs/reference/join-methods for more details.`)
	}

	if err := apitypes.ValidateJoinMethod(apitypes.JoinMethod(joinMethod)); err != nil {
		return nil, trace.Wrap(err, "Invalid Join Method")
	}
	botConfig := &embeddedtbot.BotConfig{
		AuthServer: addr,
		Onboarding: tbotconfig.OnboardingConfig{
			TokenValue: joinToken,
			CAPath:     caPath,
			JoinMethod: apitypes.JoinMethod(joinMethod),
			Terraform: tbotconfig.TerraformOnboardingConfig{
				AudienceTag: audienceTag,
			},
		},
		CredentialLifetime: tbotconfig.CredentialLifetime{
			TTL:             time.Hour,
			RenewalInterval: 20 * time.Minute,
		},
	}
	// slog default logger has been configured during the provider init.
	bot, err := embeddedtbot.New(botConfig, slog.Default())
	if err != nil {
		return nil, trace.Wrap(err, "Failed to create bot configuration, this is a provider bug, please open a GitHub issue.")
	}

	preflightCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	_, err = bot.Preflight(preflightCtx)
	if err != nil {
		return nil, trace.Wrap(err, "Failed to preflight bot configuration")
	}

	creds, err := bot.StartAndWaitForCredentials(ctx, 20*time.Second /* deadline */)
	return creds, trace.Wrap(err, "Waiting for bot to obtain credentials")
}

// activeReason renders a user-friendly active reason message describing if the credentials source is active
// and which parameters are controlling its activity.
func activeReason(active bool, params ...string) (bool, string) {
	sb := new(strings.Builder)
	var firstConjunction, lastConjunction string

	switch active {
	case true:
		firstConjunction = "either "
		lastConjunction = "or "
	case false:
		firstConjunction = "neither "
		lastConjunction = "nor "
	}

	sb.WriteString(firstConjunction)

	for i, item := range params {
		switch i {
		case len(params) - 1:
			sb.WriteString(lastConjunction)
			sb.WriteString(item)
			sb.WriteRune(' ')
		default:
			sb.WriteString(item)
			sb.WriteString(", ")
		}

	}
	sb.WriteString("are set")
	return active, sb.String()
}
