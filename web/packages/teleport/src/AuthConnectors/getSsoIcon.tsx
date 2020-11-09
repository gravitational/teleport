/*
Copyright 2020 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';
import * as Icons from 'design/Icon';
import Image from 'design/Image';
import { AuthProviderType } from 'shared/services';

const samlSvg = require('./saml-logo.svg');

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
