export type Resource = {
  id: string;
  kind: ResourceKind;
  name: string;
  displayName: string;
  content: string;
};

export type ResourceKind =
  | 'saml'
  | 'oidc'
  | 'github'
  | 'role'
  | 'auth_connector'
  | 'trusted_cluster';
