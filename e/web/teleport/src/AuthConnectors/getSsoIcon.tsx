import React from 'react';
import { Box } from 'design';
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
          color="text.main"
          {...props}
        />
      ),
      desc,
    };
  }

  if (kind === 'saml') {
    return {
      SsoIcon: props => (
        <Box {...props} height="48px">
          <SamlIcon height={48} width={100} />
        </Box>
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
        color="text.main"
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
