import React from 'react';
import * as Icons from 'design/Icon';
import Image from 'design/Image';
import { AuthProviderTypeEnum } from 'shared/services/enums';
import samlSvg from './saml-logo.svg';

export default function getSsoIcon(kind) {
  const desc = formatConnectorTypeDesc(kind);

  if (kind === AuthProviderTypeEnum.GITHUB) {
    return {
      SsoIcon: props => (
        <Icons.Github
          style={{ textAlign: 'center' }}
          fontSize="50px"
          color="text.primary"
          {...props}
        />
      ),
      desc,
    };
  }

  if (kind === AuthProviderTypeEnum.SAML) {
    return {
      SsoIcon: props => (
        <Image height="50px" width="100px" src={samlSvg} {...props} />
      ),
      desc,
    };
  }

  // default is OIDC icon
  return {
    SsoIcon: props => (
      <Icons.OpenID
        style={{ textAlign: 'center' }}
        fontSize="50px"
        color="text.primary"
        {...props}
      />
    ),
    desc,
  };
}

function formatConnectorTypeDesc(kind) {
  kind = kind || '';
  kind = kind.toUpperCase();
  return `${kind} Connector`;
}

export { AuthProviderTypeEnum };
