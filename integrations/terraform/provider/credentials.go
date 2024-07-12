package provider

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/trace"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"strings"
	"time"
)

var supportedCredentialSources = CredentialSources{
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
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic(
			"No active credentials source found",
			inactiveReason.String(),
		)}
	}
	return activeSources, nil
}

// BuildClient sequentially builds credentials for every source and tries to use them to connect to Teleport.
// Any CredentialSource failing to return a Credential and a tls.Config causes a hard failure.
// If we have a valid credential but cannot connect, we send a warning and continue with the next credential
// (this is for backward compatibility).
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
				fmt.Sprintf("Failed to build credentials %s", source.Name()),
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
				fmt.Sprintf("Failed to build credentials %s", source.Name()),
				brokenCredentialErrorSummary(source.Name(), reason, err),
			)

			return nil, diags
		}

		now := time.Now()
		if expiry, ok := creds.Expiry(); ok && !expiry.IsZero() && expiry.Before(now) {
			diags.AddWarning(
				fmt.Sprintf("Credential %s is expired", source.Name()),
				fmt.Sprintf(`The credentials %s is expired. Expiration is %q while current time is %q).
You might need to refresh them. The provider will still attempt to connect with them, but it will likely fail.`,
					source.Name(), expiry.Local(), now.Local()),
			)
		}

		clientCfg.Credentials = []client.Credentials{creds}
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
	diags.AddError("Impossible to build client", s.failedToBuildClientErrorSummary(clientCfg.Addrs[0]))
	return nil, diags
}

func (s CredentialSources) failedToBuildClientErrorSummary(addr string) string {
	sb := strings.Builder{}
	sb.WriteString("Every credential source provided has failed. The Terraform cannot connect to the Teleport cluster '")
	sb.WriteString(addr)
	sb.WriteString("'.\n")
	sb.WriteRune('\n')
	sb.WriteString("We tried building a client:\n")
	for _, source := range s {
		sb.WriteString(" - ")
		sb.WriteString(source.Name())
		sb.WriteRune('\n')
	}
	sb.WriteRune('\n')
	sb.WriteString("You can find more information about each credential source specific errors in the Terraform warnings above this error.")
	return sb.String()
}

func brokenCredentialErrorSummary(name, activeReason string, err error) string {
	sb := strings.Builder{}
	sb.WriteString("The Terraform provider tried to build credentials ")
	sb.WriteString(name)
	sb.WriteString(" but received the following error:\n\n")
	sb.WriteString(err.Error())
	sb.WriteString("\n\nThe provider tried to use the credential source because ")
	sb.WriteString(activeReason)
	sb.WriteString(". You must either address the error or disable the credential source by removing its values.")
	return sb.String()
}

type CredentialSource interface {
	Name() string
	IsActive(providerData) (bool, string)
	Credentials(context.Context, providerData) (client.Credentials, error)
}

type CredentialsFromKeyAndCertPath struct{}

func (CredentialsFromKeyAndCertPath) Name() string {
	return "from Key, Cert, and CA path"
}

func (CredentialsFromKeyAndCertPath) IsActive(config providerData) (bool, string) {
	certPath := stringFromConfigOrEnv(config.CertPath, constants.EnvVarTerraformCertificates, "")
	keyPath := stringFromConfigOrEnv(config.KeyPath, constants.EnvVarTerraformKey, "")

	// This method is active as soon as a cert or a key path are set.
	if certPath == "" && keyPath == "" {
		return false, "neither cert_path, key_path, TF_TELEPORT_CERT nor TF_TELEPORT_KEY are set"
	}

	return true, "at least one of cert_path, key_path, TF_TELEPORT_CERT nor TF_TELEPORT_KEY is set"
}

func (CredentialsFromKeyAndCertPath) Credentials(ctx context.Context, config providerData) (client.Credentials, error) {
	certPath := stringFromConfigOrEnv(config.CertPath, constants.EnvVarTerraformCertificates, "")
	keyPath := stringFromConfigOrEnv(config.KeyPath, constants.EnvVarTerraformKey, "")
	caPath := stringFromConfigOrEnv(config.RootCaPath, constants.EnvVarTerraformRootCertificates, "")

	// Validate that we have all paths.
	if certPath == "" {
		return nil, trace.BadParameter("missing parameter 'cert_path' or environment variable 'TF_TELEPORT_CERT'")
	}
	if keyPath == "" {
		return nil, trace.BadParameter("missing parameter 'key_path' or environment variable 'TF_TELEPORT_KEY'")
	}
	if caPath == "" {
		return nil, trace.BadParameter("missing parameter 'root_ca_path' or environment variable 'TF_TELEPORT_ROOT_CA'")
	}

	// Validate the files exist for a better UX?

	creds := client.LoadKeyPair(certPath, keyPath, caPath)
	return creds, nil
}

type CredentialsFromKeyAndCertBase64 struct{}

func (CredentialsFromKeyAndCertBase64) Name() string {
	return "from Key, Cert, and CA base64"
}

func (CredentialsFromKeyAndCertBase64) IsActive(config providerData) (bool, string) {
	certBase64 := stringFromConfigOrEnv(config.CertBase64, constants.EnvVarTerraformCertificatesBase64, "")
	keyBase64 := stringFromConfigOrEnv(config.KeyBase64, constants.EnvVarTerraformKeyBase64, "")

	// This method is active as soon as a cert or a key path are set.
	if certBase64 == "" && keyBase64 == "" {
		return false, "neither cert_base64, key_base64, TF_TELEPORT_CERT_BASE64 nor TF_TELEPORT_KEY_BASE64 are set"
	}

	return true, "at least one of cert_base64, key_base64, TF_TELEPORT_CERT_BASE64 nor TF_TELEPORT_KEY_BASE64 is set"
}

func (CredentialsFromKeyAndCertBase64) Credentials(ctx context.Context, config providerData) (client.Credentials, error) {
	certBase64 := stringFromConfigOrEnv(config.CertBase64, constants.EnvVarTerraformCertificatesBase64, "")
	keyBase64 := stringFromConfigOrEnv(config.KeyBase64, constants.EnvVarTerraformKeyBase64, "")
	caBase64 := stringFromConfigOrEnv(config.RootCaBase64, constants.EnvVarTerraformRootCertificatesBase64, "")

	// Validate that we have all paths.
	if certBase64 == "" {
		return nil, trace.BadParameter("missing parameter 'cert_base64' or environment variable 'TF_TELEPORT_CERT_BASE64'")
	}
	if keyBase64 == "" {
		return nil, trace.BadParameter("missing parameter 'key_base64' or environment variable 'TF_TELEPORT_KEY_BASE64'")
	}
	if caBase64 == "" {
		return nil, trace.BadParameter("missing parameter 'root_ca_base64' or environment variable 'TF_TELEPORT_ROOT_CA_BASE64'")
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

type CredentialsFromIdentityFilePath struct{}

func (CredentialsFromIdentityFilePath) Name() string {
	return "from the identity file path"
}

func (CredentialsFromIdentityFilePath) IsActive(config providerData) (bool, string) {
	identityFilePath := stringFromConfigOrEnv(config.IdentityFilePath, constants.EnvVarTerraformIdentityFilePath, "")

	// This method is active as soon as a cert or a key path are set.
	if identityFilePath == "" {
		return false, "neither identity_file_path nor TF_TELEPORT_IDENTITY_FILE_PATH are set"
	}

	return true, "either identity_file_path or TF_TELEPORT_IDENTITY_FILE_PATH is set"
}

func (CredentialsFromIdentityFilePath) Credentials(ctx context.Context, config providerData) (client.Credentials, error) {
	identityFilePath := stringFromConfigOrEnv(config.IdentityFilePath, constants.EnvVarTerraformIdentityFilePath, "")

	return client.LoadIdentityFile(identityFilePath), nil
}

type CredentialsFromIdentityFileString struct{}

func (CredentialsFromIdentityFileString) Name() string {
	return "from the identity file (passed as a string)"
}

func (CredentialsFromIdentityFileString) IsActive(config providerData) (bool, string) {
	identityFileString := stringFromConfigOrEnv(config.IdentityFile, constants.EnvVarTerraformIdentityFile, "")

	// This method is active as soon as a cert or a key path are set.
	if identityFileString == "" {
		return false, "neither identity_file nor TF_TELEPORT_IDENTITY_FILE are set"
	}

	return true, "either identity_file or TF_TELEPORT_IDENTITY_FILE is set"
}

func (CredentialsFromIdentityFileString) Credentials(ctx context.Context, config providerData) (client.Credentials, error) {
	identityFileString := stringFromConfigOrEnv(config.IdentityFile, constants.EnvVarTerraformIdentityFile, "")

	return client.LoadIdentityFileFromString(identityFileString), nil
}

type CredentialsFromIdentityFileBase64 struct{}

func (CredentialsFromIdentityFileBase64) Name() string {
	return "from the identity file (passed as a base64-encoded string)"
}

func (CredentialsFromIdentityFileBase64) IsActive(config providerData) (bool, string) {
	identityFileBase64 := stringFromConfigOrEnv(config.IdentityFileBase64, constants.EnvVarTerraformIdentityFileBase64, "")

	// This method is active as soon as a cert or a key path are set.
	if identityFileBase64 == "" {
		return false, "neither identity_file_base64 nor TF_TELEPORT_IDENTITY_FILE_BASE64 are set"
	}

	return true, "either identity_file_base64 or TF_TELEPORT_IDENTITY_FILE_BASE64 is set"
}

func (CredentialsFromIdentityFileBase64) Credentials(ctx context.Context, config providerData) (client.Credentials, error) {
	identityFileBase64 := stringFromConfigOrEnv(config.IdentityFileBase64, constants.EnvVarTerraformIdentityFileBase64, "")

	identityFile, err := base64.StdEncoding.DecodeString(identityFileBase64)
	if err != nil {
		return nil, trace.Wrap(err, "decoding base64 identity file")
	}

	return client.LoadIdentityFileFromString(string(identityFile)), nil
}

type CredentialsFromProfile struct{}

func (CredentialsFromProfile) Name() string {
	return "from the local profile"
}

func (CredentialsFromProfile) IsActive(config providerData) (bool, string) {
	profileName := stringFromConfigOrEnv(config.ProfileName, constants.EnvVarTerraformProfileName, "")
	profileDir := stringFromConfigOrEnv(config.ProfileDir, constants.EnvVarTerraformProfilePath, "")

	// This method is active as soon as a cert or a key path are set.
	if profileDir == "" && profileName == "" {
		return false, "neither profile_name, profile_dir, TF_TELEPORT_PROFILE_NAME or TF_TELEPORT_PROFILE_PATH are set"
	}

	return true, "either profile_name, profile_dir, TF_TELEPORT_PROFILE_NAME or TF_TELEPORT_PROFILE_PATH are set"
}

func (CredentialsFromProfile) Credentials(ctx context.Context, config providerData) (client.Credentials, error) {
	profileName := stringFromConfigOrEnv(config.ProfileName, constants.EnvVarTerraformProfileName, "")
	profileDir := stringFromConfigOrEnv(config.ProfileDir, constants.EnvVarTerraformProfilePath, "")

	return client.LoadProfile(profileDir, profileName), nil
}
