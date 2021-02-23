export type Resource<T extends Kind> = {
  id: string;
  kind: T;
  name: string;
  // content is config in yaml format.
  content: string;
};

export type KindRole = 'role';
export type KindTrustedCluster = 'trusted_cluster';
export type KindAuthConnectors = 'github' | 'saml' | 'oidc';
export type Kind = KindRole | KindTrustedCluster | KindAuthConnectors;
