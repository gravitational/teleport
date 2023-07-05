/*
Copyright 2022 Gravitational, Inc.

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

import React, { useState } from 'react';
import { Info, Warning } from 'design/Icon';
import Flex from 'design/Flex';

import { Notification } from './Notification';

export default {
  title: 'Shared/Notification',
};

export const Notifications = () => {
  return (
    <div
      css={`
        display: grid;
        grid-gap: ${props => props.theme.space[8]}px;
        grid-template-columns: auto auto auto;
      `}
    >
      <Flex flexDirection="column" gap={4}>
        <Notification
          item={{
            id: crypto.randomUUID(),
            severity: 'info',
            content: {
              title: 'Info with title and description',
              description: loremIpsum,
            },
          }}
          Icon={Info}
          getColor={theme => theme.colors.info}
          onRemove={() => {}}
          isAutoRemovable={false}
        />

        <Notification
          item={{
            id: crypto.randomUUID(),
            severity: 'warn',
            content: {
              title: 'Warning with title and description',
              description: loremIpsum,
            },
          }}
          Icon={Warning}
          getColor={theme => theme.colors.warning.main}
          onRemove={() => {}}
          isAutoRemovable={false}
        />

        <Notification
          item={{
            id: crypto.randomUUID(),
            severity: 'error',
            content: {
              title: 'Error with title and description',
              description: loremIpsum,
            },
          }}
          Icon={Warning}
          getColor={theme => theme.colors.danger}
          onRemove={() => {}}
          isAutoRemovable={false}
        />
      </Flex>

      <Flex flexDirection="column" gap={4}>
        <Notification
          item={{
            id: crypto.randomUUID(),
            severity: 'info',
            content: 'Multiline info without title. ' + loremIpsum,
          }}
          Icon={Info}
          getColor={theme => theme.colors.info}
          onRemove={() => {}}
          isAutoRemovable={false}
        />

        <Notification
          item={{
            id: crypto.randomUUID(),
            severity: 'warn',
            content: 'Multiline warning without title. ' + loremIpsum,
          }}
          Icon={Warning}
          getColor={theme => theme.colors.warning.main}
          onRemove={() => {}}
          isAutoRemovable={false}
        />

        <Notification
          item={{
            id: crypto.randomUUID(),
            severity: 'error',
            content: 'Multiline error without title. ' + loremIpsum,
          }}
          Icon={Warning}
          getColor={theme => theme.colors.danger}
          onRemove={() => {}}
          isAutoRemovable={false}
        />
      </Flex>

      <Flex flexDirection="column" gap={4}>
        <Notification
          item={{
            id: crypto.randomUUID(),
            severity: 'info',
            content: 'Info without title',
          }}
          Icon={Info}
          getColor={theme => theme.colors.info}
          onRemove={() => {}}
          isAutoRemovable={false}
        />

        <Notification
          item={{
            id: crypto.randomUUID(),
            severity: 'warn',
            content: 'Warning without title',
          }}
          Icon={Warning}
          getColor={theme => theme.colors.warning.main}
          onRemove={() => {}}
          isAutoRemovable={false}
        />

        <Notification
          item={{
            id: crypto.randomUUID(),
            severity: 'error',
            content: 'Error without title',
          }}
          Icon={Warning}
          getColor={theme => theme.colors.danger}
          onRemove={() => {}}
          isAutoRemovable={false}
        />
      </Flex>

      <Flex flexDirection="column" gap={4}>
        <Notification
          item={{
            id: crypto.randomUUID(),
            severity: 'info',
            content: {
              title: 'Info with link',
              description: loremIpsum,
              link: {
                href: 'https://goteleport.com',
                text: 'goteleport.com',
              },
            },
          }}
          Icon={Info}
          getColor={theme => theme.colors.info}
          onRemove={() => {}}
          isAutoRemovable={false}
        />
        <Notification
          item={{
            id: crypto.randomUUID(),
            severity: 'warn',
            content: {
              title: 'Warning with link',
              description: loremIpsum,
              link: {
                href: 'https://goteleport.com',
                text: 'goteleport.com',
              },
            },
          }}
          Icon={Warning}
          getColor={theme => theme.colors.warning.main}
          onRemove={() => {}}
          isAutoRemovable={false}
        />
        <Notification
          item={{
            id: crypto.randomUUID(),
            severity: 'error',
            content: {
              title: 'Error with link',
              description: loremIpsum,
              link: {
                href: 'https://goteleport.com',
                text: 'goteleport.com',
              },
            },
          }}
          Icon={Warning}
          getColor={theme => theme.colors.danger}
          onRemove={() => {}}
          isAutoRemovable={false}
        />
      </Flex>

      <Flex flexDirection="column" gap={4}>
        <Notification
          item={{
            id: crypto.randomUUID(),
            severity: 'info',
            content: {
              title: 'Info with list',
              list: [loremIpsum, loremIpsum],
            },
          }}
          Icon={Info}
          getColor={theme => theme.colors.info}
          onRemove={() => {}}
          isAutoRemovable={false}
        />
        <Notification
          item={{
            id: crypto.randomUUID(),
            severity: 'warn',
            content: {
              title: 'Warning with list',
              list: [loremIpsum, loremIpsum],
            },
          }}
          Icon={Warning}
          getColor={theme => theme.colors.warning.main}
          onRemove={() => {}}
          isAutoRemovable={false}
        />
        <Notification
          item={{
            id: crypto.randomUUID(),
            severity: 'error',
            content: {
              title: 'Error with list',
              list: [loremIpsum, loremIpsum],
            },
          }}
          Icon={Warning}
          getColor={theme => theme.colors.danger}
          onRemove={() => {}}
          isAutoRemovable={false}
        />
      </Flex>

      <Flex flexDirection="column" gap={4}>
        <Notification
          item={{
            id: crypto.randomUUID(),
            severity: 'info',
            content:
              'Unbreakable text. /Users/test/Library/ApplicationSupport/Electron/configuration.json',
          }}
          Icon={Info}
          getColor={theme => theme.colors.info}
          onRemove={() => {}}
          isAutoRemovable={false}
        />
      </Flex>
    </div>
  );
};

export const AutoRemovable = () => {
  const [showInfo, setShowInfo] = useState(true);
  const [showWarning, setShowWarning] = useState(true);
  const [showError, setShowError] = useState(true);
  return (
    <Flex flexDirection="column" gap={4}>
      {showInfo ? (
        <Notification
          item={{
            id: crypto.randomUUID(),
            severity: 'info',
            content:
              "This will be automatically removed after 5 seconds. Click to expand it. Mouseover it to restart the timer. Lorem Ipsum is simply dummy text of the printing and typesetting industry. Lorem Ipsum has been the industry's standard dummy text ever since the 1500s.",
          }}
          onRemove={() => setShowInfo(false)}
          Icon={Info}
          getColor={theme => theme.colors.info}
          isAutoRemovable={true}
          autoRemoveDurationMs={5000}
        />
      ) : (
        <div>Info notification has been removed</div>
      )}
      {showWarning ? (
        <Notification
          item={{
            id: crypto.randomUUID(),
            severity: 'warn',
            content:
              "This will be automatically removed after 5 seconds. Click to expand it. Mouseover it to restart the timer. Lorem Ipsum is simply dummy text of the printing and typesetting industry. Lorem Ipsum has been the industry's standard dummy text ever since the 1500s.",
          }}
          onRemove={() => setShowWarning(false)}
          Icon={Warning}
          getColor={theme => theme.colors.warning.main}
          isAutoRemovable={true}
          autoRemoveDurationMs={5000}
        />
      ) : (
        <div>Warning notification has been removed</div>
      )}
      {showError ? (
        <Notification
          item={{
            id: crypto.randomUUID(),
            severity: 'error',
            content:
              "This can only be removed by clicking on the X. Lorem Ipsum is simply dummy text of the printing and typesetting industry. Lorem Ipsum has been the industry's standard dummy text ever since the 1500s.",
          }}
          onRemove={() => setShowError(false)}
          Icon={Warning}
          getColor={theme => theme.colors.danger}
          isAutoRemovable={false}
        />
      ) : (
        <div>Error notification has been removed</div>
      )}
    </Flex>
  );
};

const loremIpsum =
  'Lorem ipsum dolor sit amet, consectetur adipiscing elit. Ut non ipsum dignissim, dignissim est vitae, facilisis nunc.';
