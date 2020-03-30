/*
Copyright 2019 Gravitational, Inc.

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

import * as Icons from 'design/Icon';
import { AuthProviderTypeEnum } from '../../services/enums';

export const TypeEnum = {
  MICROSOFT: 'microsoft',
  GITHUB: 'github',
  BITBUCKET: 'bitbucket',
  GOOGLE: 'google',
};

export function pickSsoIcon(type) {
  switch (type) {
    case TypeEnum.MICROSOFT:
      return { color: '#2672ec', Icon: Icons.Windows, type };
    case TypeEnum.GITHUB:
      return { color: '#444444', Icon: Icons.Github, type };
    case TypeEnum.BITBUCKET:
      return { color: '#205081', Icon: Icons.BitBucket, type };
    case TypeEnum.GOOGLE:
      return { color: '#dd4b39', Icon: Icons.Google, type };
    default:
      // provide default icon for unknown social providers
      return { color: '#f7931e', Icon: Icons.OpenID };
  }
}

export function guessProviderType(name, ssoType) {
  name = name.toLowerCase();

  if (name.indexOf(TypeEnum.MICROSOFT) !== -1) {
    return TypeEnum.MICROSOFT;
  }

  if (name.indexOf(TypeEnum.BITBUCKET) !== -1) {
    return TypeEnum.BITBUCKET;
  }

  if (name.indexOf(TypeEnum.GOOGLE) !== -1) {
    return TypeEnum.GOOGLE;
  }

  if (
    name.indexOf(TypeEnum.GITHUB) !== -1 ||
    ssoType === AuthProviderTypeEnum.GITHUB
  ) {
    return TypeEnum.GITHUB;
  }

  if (ssoType === AuthProviderTypeEnum.OIDC) {
    return 'openid';
  }

  return '--unknown';
}
