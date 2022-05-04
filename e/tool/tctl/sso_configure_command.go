package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/tool/tctl/flags"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/kingpin"
	"github.com/gravitational/trace"

	"github.com/sirupsen/logrus"
)

// SSOConfigureCommand implements common.CLICommand interface
type SSOConfigureCommand struct {
	config       *service.Config
	configureCmd *kingpin.CmdClause
	authCommands []*authKindCommand
	logger       *logrus.Entry
}

type authKindCommand struct {
	parsed bool
	run    func(clt auth.ClientI) error
}

// Initialize allows a caller-defined command to plug itself into CLI
// argument parsing
func (cmd *SSOConfigureCommand) Initialize(app *kingpin.Application, cfg *service.Config) {
	cmd.config = cfg
	cmd.logger = cfg.Log.WithField(trace.Component, teleport.ComponentClient)

	sso := app.GetCommand("sso")
	if sso == nil {
		sso = app.Command("sso", "Operations on auth connectors")
	}
	cmd.configureCmd = sso.Command("configure", "Create auth connector configuration.")

	cmd.authCommands = []*authKindCommand{
		addSAMLCommand(cmd),
		addOIDCCommand(cmd),
		addGithubCommand(cmd),
	}
}

// TryRun is executed after the CLI parsing is done. The command must
// determine if selectedCommand belongs to it and return match=true
func (cmd *SSOConfigureCommand) TryRun(selectedCommand string, clt auth.ClientI) (match bool, err error) {
	for _, subCommand := range cmd.authCommands {
		if subCommand.parsed {
			// the default tctl logging behaviour is to ignore all logs, unless --debug is present.
			// we want different behaviour: log messages as normal, but with compact format (no time, no caller info).
			if !cmd.config.Debug {
				formatter := utils.NewDefaultTextFormatter(trace.IsTerminal(os.Stderr))
				formatter.FormatCaller = func() (caller string) { return "" }
				cmd.logger.Logger.SetFormatter(formatter)
				cmd.logger.Logger.SetOutput(os.Stderr)
			}

			return true, trace.Wrap(subCommand.run(clt))
		}
	}

	return false, nil
}

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

// TODO(Tener): move to common package once Github (OSS) part is added.
// TODO(Tener): use json.Indent() to implement.
func indent(lines string, indent string) string {

	builder := strings.Builder{}

	for _, line := range strings.Split(lines, "\n") {
		builder.WriteString(indent)
		builder.WriteString(line)
		builder.WriteString("\n")
	}

	return strings.TrimSuffix(builder.String(), "\n")
}

type samlExtraFlags struct {
	chosenPreset         string
	connectorName        string
	ignoreMissingRoles   bool
	entityDescriptorFlag string
	signingKeyPair       types.AsymmetricKeyPair
	encryptionKeyPair    types.AsymmetricKeyPair
}

func addSAMLCommand(cmd *SSOConfigureCommand) *authKindCommand {
	spec := types.SAMLConnectorSpecV2{}

	pTable := asciitable.MakeTable([]string{"Name", "Description", "Display"})
	for _, preset := range samlPresets {
		pTable.AddRow([]string{preset.name, preset.description, preset.display})
	}
	presets := indent(pTable.AsBuffer().String(), "  ")

	sub := cmd.configureCmd.Command("saml", fmt.Sprintf("Configure SAML connector, optionally using a preset. Available presets: %v", samlPresets.getNames()))

	saml := &samlExtraFlags{}

	// commonly used flags
	sub.Flag("preset", fmt.Sprintf("Preset. One of: %v", samlPresets.getNames())).Short('p').EnumVar(&saml.chosenPreset, samlPresets.getNames()...)
	sub.Flag("name", "Connector name. Required, unless implied from preset.").Short('n').StringVar(&saml.connectorName)
	sub.Flag("entity-descriptor", "Set the Entity Descriptor. Valid values: file, URL, XML content. Supplies configuration parameters as single XML instead of individual elements.").Short('e').StringVar(&saml.entityDescriptorFlag)
	sub.Flag("attributes-to-roles", "Sets attribute-to-role mapping in the form 'attr_name,attr_value,role1,role2,...'. Repeatable.").Short('a').Required().SetValue(flags.NewAttributesToRolesParser(&spec.AttributesToRoles))
	sub.Flag("display", "Display controls how this connector is displayed.").StringVar(&spec.Display)

	// alternatives to --entity-descriptor:
	sub.Flag("issuer", "Issuer is the identity provider issuer.").StringVar(&spec.Issuer)
	sub.Flag("sso", "SSO is the URL of the identity provider's SSO service.").StringVar(&spec.SSO)
	sub.Flag("cert", "Cert is the identity provider certificate PEM. IDP signs <Response> responses using this certificate.").StringVar(&spec.Cert)
	sub.Flag("cert-file", "Like --cert, but read the cert from file.").SetValue(flags.NewFileReader(&spec.Cert))

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
	sub.Flag("ignore-missing-roles", "Ignore non-existing roles referenced in --attributes-to-roles.").BoolVar(&saml.ignoreMissingRoles)

	sub.Alias(fmt.Sprintf(`
Presets:

%vExamples:

  > tctl sso configure saml -n myauth -a groups,admin,access,editor,audit -a group,developer,access -e entity-desc.xml  

  Generate SAML auth connector configuration named 'myauth'. Two mappings from SAML attributes to roles are defined: 
    - members of 'admin' group will receive 'access', 'editor' and 'audit' role.
    - members of 'developer' group will receive 'access' role.
  The IdP metadata will be read from 'entity-desc.xml' file.


  > tctl sso configure saml -p okta -a group,dev,access -e https://dev-123456.oktapreview.com/app/ex30h8/sso/saml/metadata

  Generate SAML auth connector configuration using 'okta' preset. The choice of preset affects default name, display attribute and may apply IdP-specific tweaks.
  Instead of XML file, a URL was provided to -e flag, which will be fetched by Teleport during runtime.


  > tctl sso configure saml -p okta -a group,developer,access -e entity-desc.xml | tctl sso test
  
  Generate the configuration and immediately test it using "tctl sso test" command.

`, presets))

	preset := &authKindCommand{
		parsed: false,
		run:    samlRunFunc(cmd, &spec, saml),
	}

	sub.Action(func(ctx *kingpin.ParseContext) error {
		preset.parsed = true
		return nil
	})

	return preset
}

func samlRunFunc(cmd *SSOConfigureCommand, spec *types.SAMLConnectorSpecV2, flags *samlExtraFlags) func(clt auth.ClientI) error {
	return func(clt auth.ClientI) error {
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

		allRoles, err := clt.GetRoles(context.Background())
		if err != nil {
			cmd.logger.WithError(err).Warn("unable to get roles list. Skipping attributes-to-roles sanity checks.")
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
							cmd.logger.Warnf("attributes-to-roles references non-existing role: %q. Available roles: %v.", role, roleNames)
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
			cmd.logger.Info("ACS empty, resolving automatically.")
			proxies, err := clt.GetProxies()
			if err != nil {
				cmd.logger.WithError(err).Warn("unable to get proxy list.")
			}

			// find first proxy with public addr
			for _, proxy := range proxies {
				publicAddr := proxy.GetPublicAddr()
				if publicAddr != "" {
					spec.AssertionConsumerService = fmt.Sprintf("https://%v/v1/webapi/saml/acs", publicAddr)
					break
				}
			}

			// check if successfully set.
			if spec.AssertionConsumerService == "" {
				cmd.logger.Warn("Unable to resolve ACS automatically: cluster's public address unknown.")
			} else {
				cmd.logger.Infof("ACS set to %q", spec.AssertionConsumerService)
			}
		}

		// figure out the actual meaning of entityDescriptorFlag. Can be: URL, file, plain XML.
		if flags.entityDescriptorFlag != "" {
			if err = processEntityDescriptorFlag(spec, flags.entityDescriptorFlag, cmd.logger); err != nil {
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
}

// keyPairFromFlags is a helper func to set key pair if appropriate flags were given.
func keyPairFromFlags(flags types.AsymmetricKeyPair) *types.AsymmetricKeyPair {
	if flags.PrivateKey == "" && flags.Cert == "" {
		return nil
	}

	return &flags
}

func processEntityDescriptorFlag(spec *types.SAMLConnectorSpecV2, entityDescriptorFlag string, log *logrus.Entry) error {
	var err error

	// case: URL
	var parsedURL *url.URL
	if parsedURL, err = url.Parse(entityDescriptorFlag); err == nil && parsedURL.Scheme != "" {
		spec.EntityDescriptorURL = entityDescriptorFlag
		log.Infof("Entity descriptor looks like URL, entity-descriptor-url set to %q.", spec.EntityDescriptorURL)
		return nil
	}
	if parsedURL.Scheme == "" {
		log.Infof("Cannot parse entity descriptor as URL, missing scheme: %q.", entityDescriptorFlag)
	} else {
		log.WithError(err).Infof("Cannot parse entity descriptor as URL: %q.", entityDescriptorFlag)
	}

	// case: file
	var bytes []byte
	if bytes, err = os.ReadFile(entityDescriptorFlag); err == nil {
		if err = validateEntityDescriptor(bytes, spec.Cert); err != nil {
			return trace.WrapWithMessage(err, "Validating entity descriptor from file %q failed. Check that XML is valid or download the file directly.", entityDescriptorFlag)
		}
		spec.EntityDescriptor = string(bytes)
		log.Infof("Entity descriptor read from file %q.", entityDescriptorFlag)
		return nil
	}
	log.WithError(err).Infof("Cannot read entity descriptor from file: %q.", entityDescriptorFlag)

	// case: verbatim XML
	if err = validateEntityDescriptor([]byte(entityDescriptorFlag), spec.Cert); err == nil {
		spec.EntityDescriptor = entityDescriptorFlag
		log.Infof("Entity descriptor is valid XML, EntityDescriptor set to flag value.")
		return nil
	}
	log.WithError(trace.Unwrap(err)).Infof("Cannot parse entity descriptor as verbatim XML: %q.", entityDescriptorFlag)

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

func addGithubCommand(cmd *SSOConfigureCommand) *authKindCommand {
	sub := cmd.configureCmd.Command("github", "Configure GitHub auth connector.")
	preset := &authKindCommand{
		parsed: false,
		run: func(clt auth.ClientI) error {
			return trace.NotImplemented("GitHub not yet implemented.")
		},
	}

	sub.Action(func(ctx *kingpin.ParseContext) error {
		preset.parsed = true
		return nil
	})

	return preset
}

func addOIDCCommand(cmd *SSOConfigureCommand) *authKindCommand {
	sub := cmd.configureCmd.Command("oidc", "Configure OIDC auth connector, optionally using a preset.")
	preset := &authKindCommand{
		parsed: false,
		run: func(clt auth.ClientI) error {
			return trace.NotImplemented("OIDC not yet implemented.")
		},
	}

	sub.Action(func(ctx *kingpin.ParseContext) error {
		preset.parsed = true
		return nil
	})

	return preset
}
