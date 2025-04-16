// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package configure

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"os"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/tool/tctl/sso/configure/flags"
	"github.com/gravitational/teleport/tool/tctl/sso/tester"
)

type samlPreset struct {
	name        string
	description string
	display     string
	modifySpec  func(spec *types.SAMLConnectorSpecV2) error
}

type samlPresetList []samlPreset

func (lst samlPresetList) getNames() []string {
	var names []string
	for _, p := range lst {
		names = append(names, p.name)
	}
	return names
}

func (lst samlPresetList) getPreset(name string) *samlPreset {
	for _, p := range lst {
		if p.name == name {
			return &p
		}
	}
	return nil
}

var samlPresets = samlPresetList([]samlPreset{
	{name: "okta", description: "Okta", display: "Okta"},
	{name: "onelogin", description: "OneLogin", display: "OneLogin"},
	{name: "ad", description: "Azure Active Directory", display: "Microsoft"},
	{name: "adfs", description: "Active Directory Federation Services", display: "ADFS", modifySpec: func(spec *types.SAMLConnectorSpecV2) error {
		spec.Provider = teleport.ADFS
		return nil
	}},
})

type samlExtraFlags struct {
	chosenPreset         string
	connectorName        string
	ignoreMissingRoles   bool
	entityDescriptorFlag string
	signingKeyPair       types.AsymmetricKeyPair
	encryptionKeyPair    types.AsymmetricKeyPair
}

func addSAMLCommand(cmd *SSOConfigureCommand) *AuthKindCommand {
	spec := types.SAMLConnectorSpecV2{}

	pTable := asciitable.MakeTable([]string{"Name", "Description", "Display"})
	for _, preset := range samlPresets {
		pTable.AddRow([]string{preset.name, preset.description, preset.display})
	}
	presets := tester.Indent(pTable.AsBuffer().String(), 2)

	sub := cmd.ConfigureCmd.Command("saml", fmt.Sprintf("Configure SAML auth connector, optionally using a preset. Available presets: %v.", samlPresets.getNames()))

	saml := &samlExtraFlags{}

	// commonly used flags
	sub.Flag("preset", fmt.Sprintf("Preset. One of: %v", samlPresets.getNames())).Short('p').EnumVar(&saml.chosenPreset, samlPresets.getNames()...)
	sub.Flag("name", "Connector name. Required, unless implied from preset.").Short('n').StringVar(&saml.connectorName)
	sub.Flag("entity-descriptor", "Set the Entity Descriptor. Valid values: file, URL, XML content. Supplies configuration parameters as single XML instead of individual elements.").Short('e').StringVar(&saml.entityDescriptorFlag)
	sub.Flag("attributes-to-roles", "Sets attribute-to-role mapping using format 'attr_name,attr_value,role1,role2,...'. Repeatable.").Short('r').Required().SetValue(flags.NewAttributesToRolesParser(&spec.AttributesToRoles))
	sub.Flag("display", "Sets the connector display name.").StringVar(&spec.Display)
	sub.Flag("allow-idp-initiated", "Allow the IdP to initiate the SSO flow.").BoolVar(&spec.AllowIDPInitiated)

	// alternatives to --entity-descriptor:
	sub.Flag("issuer", "Issuer is the identity provider issuer.").StringVar(&spec.Issuer)
	sub.Flag("sso", "SSO is the URL of the identity provider's SSO service.").StringVar(&spec.SSO)
	sub.Flag("cert", "Cert file with with the IdP certificate PEM. IdP signs <Response> responses using this certificate.").SetValue(flags.NewFileReader(&spec.Cert))

	// provided for completeness, but typically omitted.
	sub.Flag("acs", "AssertionConsumerService is a URL for assertion consumer service on the service provider (Teleport's side).").StringVar(&spec.AssertionConsumerService)
	sub.Flag("audience", "Audience uniquely identifies our service provider.").StringVar(&spec.Audience)
	sub.Flag("service-provider-issuer", "ServiceProviderIssuer is the issuer of the service provider (Teleport).").StringVar(&spec.ServiceProviderIssuer)
	sub.Flag("signing-key-file", "A file with request signing key. Must be used together with --signing-cert-file.").SetValue(flags.NewFileReader(&saml.signingKeyPair.PrivateKey))
	sub.Flag("signing-cert-file", "A file with request certificate. Must be used together with --signing-key-file.").SetValue(flags.NewFileReader(&saml.signingKeyPair.Cert))

	// advanced feature: assertion encryption
	sub.Flag("assertion-key-file", "A file with key used for securing SAML assertions. Must be used together with --assertion-cert-file.").SetValue(flags.NewFileReader(&saml.encryptionKeyPair.PrivateKey))
	sub.Flag("assertion-cert-file", "A file with cert used for securing SAML assertions. Must be used together with --assertion-key-file.").SetValue(flags.NewFileReader(&saml.encryptionKeyPair.Cert))

	// niche: required for particular providers.
	sub.Flag("provider", "Sets the external identity provider type. Examples: ping, adfs.").StringVar(&spec.Provider)

	// ignore warnings;
	sub.Flag("ignore-missing-roles", "Ignore missing roles referenced in --attributes-to-roles.").BoolVar(&saml.ignoreMissingRoles)

	sub.Alias(fmt.Sprintf(`
Presets:

%v

Examples:

  > tctl sso configure saml -n myauth -r groups,admin,access,editor,auditor -r groups,developer,access -e entity-desc.xml

  Generate SAML auth connector configuration called 'myauth'. Two mappings from SAML attributes to roles are defined:
    - members of 'admin' group will receive 'access', 'editor' and 'auditor' roles.
    - members of 'developer' group will receive 'access' role.
  The IdP metadata will be read from 'entity-desc.xml' file.


  > tctl sso configure saml -p okta -r group,dev,access -e https://dev-123456.oktapreview.com/app/ex30h8/sso/saml/metadata

  Generate SAML auth connector configuration using 'okta' preset. The choice of preset affects default name, display attribute and may apply IdP-specific tweaks.
  Instead of XML file, a URL was provided to -e flag, which will be fetched by Teleport during runtime.


  > tctl sso configure saml -p okta -r groups,developer,access -e entity-desc.xml | tctl sso test

  Generate the configuration and immediately test it using "tctl sso test" command.

`, presets))

	preset := &AuthKindCommand{
		Run: func(ctx context.Context, clt *authclient.Client) error {
			return samlRunFunc(ctx, cmd, &spec, saml, clt)
		},
	}

	sub.Action(func(ctx *kingpin.ParseContext) error {
		preset.Parsed = true
		return nil
	})

	return preset
}

func samlRunFunc(
	ctx context.Context,
	cmd *SSOConfigureCommand,
	spec *types.SAMLConnectorSpecV2,
	flags *samlExtraFlags,
	clt *authclient.Client,
) error {
	// apply preset, if chosen
	p := samlPresets.getPreset(flags.chosenPreset)
	if p != nil {
		if spec.Display == "" {
			spec.Display = p.display
		}

		if flags.connectorName == "" {
			flags.connectorName = p.name
		}

		if p.modifySpec != nil {
			if err := p.modifySpec(spec); err != nil {
				return trace.Wrap(err)
			}
		}
	}

	if flags.connectorName == "" {
		return trace.BadParameter("Connector name must be set, either by choosing --preset or explicitly via --name")
	}

	allRoles, err := clt.GetRoles(ctx)
	if err != nil {
		cmd.Logger.WarnContext(ctx, "unable to get roles list, skipping attributes-to-roles sanity checks", "error", err)
	} else {
		roleMap := map[string]bool{}
		var roleNames []string
		for _, role := range allRoles {
			roleMap[role.GetName()] = true
			roleNames = append(roleNames, role.GetName())
		}

		for _, attrMapping := range spec.AttributesToRoles {
			for _, role := range attrMapping.Roles {
				_, found := roleMap[role]
				if !found {
					if flags.ignoreMissingRoles {
						cmd.Logger.WarnContext(ctx, "attributes-to-roles references non-existing role", "role", role, "available_roles", roleNames)
					} else {
						return trace.BadParameter("attributes-to-roles references non-existing role: %v. Correct the mapping, or add --ignore-missing-roles to ignore this error. Available roles: %v.", role, roleNames)
					}
				}
			}
		}
	}

	spec.SigningKeyPair = keyPairFromFlags(flags.signingKeyPair)
	if spec.SigningKeyPair != nil {
		if spec.SigningKeyPair.PrivateKey == "" {
			return trace.BadParameter("Signing key pair was set, but key is empty. Provide the key with --signing-key-file.")
		}
		if spec.SigningKeyPair.Cert == "" {
			return trace.BadParameter("Signing key pair was set, but cert is empty. Provide the cert with --signing-cert-file.")
		}
	}

	spec.EncryptionKeyPair = keyPairFromFlags(flags.encryptionKeyPair)
	if spec.EncryptionKeyPair != nil {
		if spec.EncryptionKeyPair.PrivateKey == "" {
			return trace.BadParameter("Assertion key pair was set, but key is empty. Provide the key with --assertion-key-file.")
		}
		if spec.EncryptionKeyPair.Cert == "" {
			return trace.BadParameter("Assertion key pair was set, but cert is empty. Provide the cert with --assertion-cert-file.")
		}
	}

	if spec.AssertionConsumerService == "" {
		spec.AssertionConsumerService = ResolveCallbackURL(ctx, cmd.Logger, clt, "ACS", "https://%v/v1/webapi/saml/acs/"+flags.connectorName)
	}

	// figure out the actual meaning of entityDescriptorFlag. Can be: URL, file, plain XML.
	if flags.entityDescriptorFlag != "" {
		if err = processEntityDescriptorFlag(ctx, spec, flags.entityDescriptorFlag, cmd.Logger); err != nil {
			return trace.Wrap(err)
		}
	}

	if spec.Cert != "" {
		if err = validateCert(spec.Cert); err != nil {
			return trace.Wrap(err, "invalid certificate provided with --cert.")
		}
	}

	if spec.EntityDescriptorURL == "" && spec.EntityDescriptor == "" && (spec.Issuer == "" || spec.SSO == "" || spec.Cert == "") {
		return trace.BadParameter("missing one or more: issuer, sso, cert. Provide missing values using corresponding flags or with Entity Descriptor -e FILE/URL/XML.")
	}

	connector, err := types.NewSAMLConnector(flags.connectorName, *spec)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(utils.WriteYAML(os.Stdout, connector))
}

// keyPairFromFlags is a helper func to set key pair if appropriate flags were given.
func keyPairFromFlags(flags types.AsymmetricKeyPair) *types.AsymmetricKeyPair {
	if flags.PrivateKey == "" && flags.Cert == "" {
		return nil
	}

	return &flags
}

func processEntityDescriptorFlag(ctx context.Context, spec *types.SAMLConnectorSpecV2, entityDescriptorFlag string, log *slog.Logger) error {
	var err error

	// case: URL
	var parsedURL *url.URL
	if parsedURL, err = url.Parse(entityDescriptorFlag); err == nil && parsedURL.Scheme != "" {
		spec.EntityDescriptorURL = entityDescriptorFlag
		log.InfoContext(ctx, "Using entity descriptor URL", "entity_descriptor_url", spec.EntityDescriptorURL)
		return nil
	}
	if parsedURL.Scheme == "" {
		log.InfoContext(ctx, "entity descriptor URL missing scheme", "entity_descriptor", entityDescriptorFlag)
	} else {
		log.InfoContext(ctx, "invalid entity descriptor URL", "entity_descriptor", entityDescriptorFlag, "error", err)
	}

	// case: file
	var bytes []byte
	if bytes, err = os.ReadFile(entityDescriptorFlag); err == nil {
		if err = validateEntityDescriptor(bytes, spec.Cert); err != nil {
			return trace.WrapWithMessage(err, "Validating entity descriptor from file %q failed. Check that XML is valid or download the file directly.", entityDescriptorFlag)
		}
		spec.EntityDescriptor = string(bytes)
		log.InfoContext(ctx, "Entity descriptor read from file", "file", entityDescriptorFlag)
		return nil
	}
	log.InfoContext(ctx, "Cannot read entity descriptor from file", "file", entityDescriptorFlag, "error", err)

	// case: verbatim XML
	if err = validateEntityDescriptor([]byte(entityDescriptorFlag), spec.Cert); err == nil {
		spec.EntityDescriptor = entityDescriptorFlag
		log.InfoContext(ctx, "Entity descriptor is valid XML, EntityDescriptor set to flag value")
		return nil
	}
	log.InfoContext(ctx, "Cannot parse entity descriptor as verbatim XML", "entity_descriptor", entityDescriptorFlag, "error", err)

	return trace.Errorf("failed to process -e/--entity-descriptor flag. Valid values: XML file, URL, verbatim XML")
}

func validateCert(certString string) error {
	_, err := tlsca.ParseCertificatePEM([]byte(certString))
	if err != nil {
		return trace.Wrap(err, "failed to parse certificate")
	}
	return nil
}

func validateEntityDescriptor(entityDescriptorXML []byte, specCert string) error {
	certificates, err := services.CheckSAMLEntityDescriptor(string(entityDescriptorXML))
	if err != nil {
		return trace.Wrap(err)
	}

	// ensure we have at least one root.
	if len(certificates) > 0 {
		return nil
	}

	if specCert == "" {
		return trace.BadParameter("no certificates in entity descriptor and none provided with --cert.")
	}

	err = validateCert(specCert)
	if err != nil {
		return trace.Wrap(err, "no certificates in entity descriptor and invalid certificate provided with --cert.")
	}

	return nil
}
