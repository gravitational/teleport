import React from 'react';
import * as Icons from 'design/Icon';
import Image from 'design/Image';
import { AuthProviderType } from 'shared/services';

const samlSvg = require('./saml-logo.svg').default;

export default function getSsoIcon(kind: AuthProviderType) {
  const desc = formatConnectorTypeDesc(kind);
  if (kind === 'github') {
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

  if (kind === 'saml') {
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
