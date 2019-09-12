import { at } from 'lodash';

export default function makeResource(json) {
  const [id, kind, name, content] = at(json, ['id', 'kind', 'name', 'content']);
  return {
    id,
    kind,
    name,
    displayName: name,
    content,
  };
}

export const ResourceEnum = {
  SAML: 'saml',
  OIDC: 'oidc',
  ROLE: 'role',
  AUTH_CONNECTORS: 'auth_connector',
  TRUSTED_CLUSTER: 'trusted_cluster',
};
