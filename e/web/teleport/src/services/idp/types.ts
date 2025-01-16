import type {
  AttributeMapping,
  SamlServiceProviderPreset,
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
