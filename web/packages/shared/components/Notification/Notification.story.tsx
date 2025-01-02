/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { useState } from 'react';

import Flex from 'design/Flex';
import { Bots } from 'design/Icon';

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
          onRemove={() => {}}
          isAutoRemovable={false}
        />

        <Notification
          item={{
            id: crypto.randomUUID(),
            severity: 'neutral',
            content: {
              title: 'Neutral with title and description',
              description: loremIpsum,
            },
          }}
          onRemove={() => {}}
          isAutoRemovable={false}
        />

        <Notification
          item={{
            id: crypto.randomUUID(),
            severity: 'neutral',
            content: {
              title: 'Custom icon with title and description',
              description: loremIpsum,
              icon: Bots,
            },
          }}
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
              title: 'Info with title and description',
              subtitle: 'And some subtitle, too',
              description: loremIpsum,
            },
          }}
          onRemove={() => {}}
          isAutoRemovable={false}
        />

        <Notification
          item={{
            id: crypto.randomUUID(),
            severity: 'warn',
            content: {
              title: 'Warning with title and description',
              subtitle: 'And some subtitle, too',
              description: loremIpsum,
            },
          }}
          onRemove={() => {}}
          isAutoRemovable={false}
        />

        <Notification
          item={{
            id: crypto.randomUUID(),
            severity: 'error',
            content: {
              title: 'Error with title and description',
              subtitle: 'And some subtitle, too',
              description: loremIpsum,
            },
          }}
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
          onRemove={() => {}}
          isAutoRemovable={false}
        />

        <Notification
          item={{
            id: crypto.randomUUID(),
            severity: 'warn',
            content: 'Multiline warning without title. ' + loremIpsum,
          }}
          onRemove={() => {}}
          isAutoRemovable={false}
        />

        <Notification
          item={{
            id: crypto.randomUUID(),
            severity: 'error',
            content: 'Multiline error without title. ' + loremIpsum,
          }}
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
              action: {
                href: 'https://goteleport.com',
                content: 'Learn More',
              },
            },
          }}
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
              'Long continuous strings. /Users/test/Library/ApplicationSupport/foobarbazio/barbazfoobarioloremoipsumoconfigurationobaziofoobazi/baz/lorem/ipsum/Electron/configuration.json',
          }}
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
              title:
                'A very long title with a very long address that spans multiple lines tcp-postgres.foo.bar.baz.cloud.gravitational.io and some more text on another line',
              description:
                'Long continuous strings. /Users/test/Library/ApplicationSupport/foobarbazio/barbazfoobarioloremoipsumoconfigurationobaziofoobazi/baz/lorem/ipsum/Electron/configuration.json',
            },
          }}
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
              description: 'Info with description, without a title',
            },
          }}
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
              list: ['Info with a list', 'But no title'],
            },
          }}
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
          isAutoRemovable={true}
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
          isAutoRemovable={true}
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
