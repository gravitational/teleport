import { AuthProviderTypeEnum } from 'shared/services/enums';
import saml from '!raw-loader!./saml.yaml';
import github from '!raw-loader!./github.yaml';
import oidc from '!raw-loader!./oidc.yaml';

export function getTemplate(kind) {
  if (kind === AuthProviderTypeEnum.SAML) {
    return saml;
  }

  if (kind === AuthProviderTypeEnum.GITHUB) {
    return github;
  }

  if (kind === AuthProviderTypeEnum.OIDC) {
    return oidc;
  }

  return '';
}
