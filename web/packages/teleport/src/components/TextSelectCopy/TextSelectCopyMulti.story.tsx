/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';

import { TextSelectCopyMulti as Component } from './TextSelectCopyMulti';

export default {
  title: 'Teleport/TextSelectCopy/Multi',
};

export const BashMulti = () => {
  return (
    <Component
      lines={[
        {
          text: `sudo tctl -c cfg-all users add --roles=access,editor george_washington`,
        },
        {
          text: 'sudo DEBUG=1 teleport start -c cfg-all -d',
        },
        {
          text: 'yarn start-teleport-e --target=https://localhost:3080/web',
        },
      ]}
    />
  );
};

export const BashMultiWithComment = () => {
  return (
    <Component
      lines={[
        {
          text: `sudo curl https://apt.releases.teleport.dev/gpg \\\n-o /usr/share/keyrings/teleport-archive-keyring.asc`,
          comment: `Download Teleport's PGP public key`,
        },
        {
          text: 'sudo DEBUG=1 teleport start -c cfg-all -d',
        },
        {
          text:
            `echo "deb [signed-by=/usr/share/keyrings/teleport-archive-keyring.asc] \\\n` +
            `https://apt.releases.teleport.dev/stable/v10" \\\n` +
            `| sudo tee /etc/apt/sources.list.d/teleport.list > /dev/null`,
          comment:
            `Add the Teleport APT repository for v10. You'll need to update this` +
            `\nfile for each major release of Teleport.\n` +
            `Note: if using a fork of Debian or Ubuntu you may need to use '$ID_LIKE'\n` +
            `and the codename your distro was forked from instead of '$ID' and '$VERSION_CODENAME'.\n`,
        },
      ]}
    />
  );
};

export const BashSingle = () => {
  return (
    <Component
      lines={[
        {
          text: `sudo tctl -c cfg-all users add --roles=access,editor george_washington`,
        },
      ]}
    />
  );
};

export const BashSingleWithComment = () => {
  return (
    <Component
      lines={[
        {
          text: `sudo tctl -c cfg-all users add --roles=access,editor george_washington`,
          comment: `Add the Teleport API repository for v10. You'll need to update this.`,
        },
      ]}
    />
  );
};

export const NonBash = () => {
  return (
    <Component
      lines={[
        {
          text: 'some -c text to be copied and it is super long to test scrolling',
        },
      ]}
      bash={false}
    />
  );
};
