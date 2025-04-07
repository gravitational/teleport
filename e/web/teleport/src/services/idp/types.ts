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
};

export type CreateSamlIdpServiceProviderResponse = {
  id: string;
  kind: string;
  name: string;
  content: string;
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
