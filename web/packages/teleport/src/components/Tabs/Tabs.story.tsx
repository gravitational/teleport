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

import { TextSelectCopyMulti } from '../TextSelectCopy';

import { Tabs } from './Tabs';

export default {
  title: 'Teleport/Tabs',
};

export const Plain = () => {
  return (
    <Tabs
      tabs={[
        {
          title: `One`,
          content: (
            <div>
              Lorem Ipsum is simply dummy text of the printing and typesetting
              industry.
            </div>
          ),
        },
        {
          title: `Two`,
          content: (
            <div>
              Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do
              eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut
              enim ad minim veniam, quis nostrud exercitation ullamco laboris
              nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in
              reprehenderit in voluptate velit esse cillum dolore eu fugiat
              nulla pariatur.
            </div>
          ),
        },
      ]}
    />
  );
};

export const PlainOneTab = () => {
  return (
    <Tabs
      tabs={[
        {
          title: `One`,
          content: (
            <div>
              Lorem Ipsum is simply dummy text of the printing and typesetting
              industry.
            </div>
          ),
        },
      ]}
    />
  );
};

export const TerminalCommands = () => {
  return (
    <Tabs
      tabs={[
        {
          title: `Debian/Ubuntu (DEB)`,
          content: (
            <TextSelectCopyMulti
              lines={[
                {
                  text: 'sudo ./tctl -c cfg-all users add --roles=admin lisa',
                  comment: 'Some kind of comment',
                },
              ]}
            />
          ),
        },
        {
          title: 'Tarball',
          content: (
            <TextSelectCopyMulti
              lines={[
                {
                  text: 'sudo ./tctl -c cfg-all users add --roles=access,admin georege',
                  comment: 'Lorem ipsum dolores',
                },
                {
                  text: 'tsh login --user=georege',
                },
              ]}
            />
          ),
        },
        {
          title: 'Debian/Ubuntu Legacy (DEB)',
          content: (
            <TextSelectCopyMulti
              lines={[
                {
                  text: 'sudo ./tctl -c cfg-all users add --roles=access,admin georege',
                },
                {
                  text: 'tsh login --user=georege',
                },
              ]}
            />
          ),
        },
        {
          title: 'Amazon Linux 2/RHEL Legacy (RPM)',
          content: (
            <TextSelectCopyMulti
              lines={[
                {
                  text: 'sudo ./tctl -c cfg-all users ls',
                },
              ]}
            />
          ),
        },
      ]}
    />
  );
};
