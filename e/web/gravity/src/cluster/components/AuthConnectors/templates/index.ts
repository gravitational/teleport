import { AuthProviderType } from 'shared/services';
const saml = require('!raw-loader!./saml.yaml').default;
const github = require('!raw-loader!./github.yaml').default;
const oidc = require('!raw-loader!./oidc.yaml').default;

export function getTemplate(kind: AuthProviderType) {
  if (kind === 'saml') {
    return saml;
  }

  if (kind === 'github') {
    return github;
  }

  if (kind === 'oidc') {
    return oidc;
  }

  return '';
}
