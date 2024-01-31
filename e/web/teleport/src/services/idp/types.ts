export type CreateSamlIdpServiceProviderRequest = {
  name: string;
  entityID: string;
  acsURL: string;
  entityDescriptor: string;
  attributeMapping?: AttributeMapping[];
};

export type AttributeMapping = {
  name: string;
  value: string;
  nameFormat?: string;
};

export type CreateSamlIdpServiceProviderResponse = {
  id: string;
  kind: string;
  name: string;
  content: string;
};

export type SAMLIdPMetadataResponse = {
  entityID: string;
  ssoURL: string;
  x509PEM: string;
};
