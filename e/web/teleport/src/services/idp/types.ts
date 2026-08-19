import {
  SamlIdpServiceProviderSpec,
  SamlServiceProviderPreset,
  type AttributeMapping,
} from 'teleport/services/samlidp/types';

export type CreateSamlIdpServiceProviderRequest = {
  name: string;
  entityID: string;
  acsURL: string;
  entityDescriptor: string;
  attributeMapping?: AttributeMapping[];
  preset?: SamlServiceProviderPreset;
  labels?: Record<string, string>;
  launchURLs?: string[];
};

export type SamlIdpMetadataResponse = {
  entityID: string;
  ssoURL: string;
  x509PEM: string;
};

/**
 * gcpWorkforcePresetSpec returns preset values for
 * GCP Workforce Identity Federation SAML service provider.
 */
export const gcpWorkforcePresetSpec = (): SamlIdpServiceProviderSpec => {
  return {
    acs_url: '',
    attribute_mapping: [
      {
        name: 'roles',
        name_format: 'unspecified',
        value: 'user.spec.roles',
      },
    ],
    entity_descriptor: '',
    entity_id: '',
    preset: SamlServiceProviderPreset.GcpWorkforce,
    relay_state: '',
  };
};

/**
 * microsoftEntraIdPresetSpec returns preset values for
 * Microsoft Entra Id SAML service provider.
 */
export const microsoftEntraIdPresetSpec = (
  tenantId?: string
): SamlIdpServiceProviderSpec => {
  return {
    acs_url: 'https://login.microsoftonline.com/login.srf',
    attribute_mapping: [
      {
        // TODO(sshah): abstract the attribute name to const as this is copied in multiple places.
        name: 'http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress',
        name_format: 'unspecified',
        value: 'uid',
      },
    ],
    entity_descriptor: '',
    entity_id: `https://login.microsoftonline.com/${tenantId}/`, // trailing slash is required
    preset: SamlServiceProviderPreset.MicrosoftEntraId,
    launch_urls: [`https://portal.azure.com/${tenantId}`],
    relay_state: '',
  };
};
