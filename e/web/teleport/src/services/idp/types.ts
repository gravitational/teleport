export type CreateSamlIdpServiceProviderRequest = {
  name: string;
  entityDescriptor: string;
};

export type CreateSamlIdpServiceProviderResponse = {
  id: string;
  kind: string;
  name: string;
  content: string;
};
