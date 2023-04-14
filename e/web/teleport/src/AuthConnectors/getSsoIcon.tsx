import React from 'react';
import * as Icons from 'design/Icon';
import { AuthProviderType } from 'shared/services';

import { SamlIcon } from './SamlIcon';

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
      SsoIcon: props => <SamlIcon height={50} width={100} {...props} />,
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
