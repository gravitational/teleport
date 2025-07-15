/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import { Meta } from '@storybook/react';

import { SlidingSidePanel } from 'shared/components/SlidingSidePanel';
import { InfoGuideContainer } from 'shared/components/SlidingSidePanel/InfoGuide';
import { resourceStatusPanelWidth } from 'shared/components/SlidingSidePanel/InfoGuide/const';
import { Attempt } from 'shared/hooks/useAttemptNext';

import {
  DatabaseServer,
  SharedResourceServer,
  UnifiedResourceDefinition,
} from '../types';
import {
  UnhealthyStatusInfo as Component,
  StatusInfoHeader,
} from './StatusInfo';

type StoryProps = {
  attemptState: 'success' | 'processing' | 'failed' | '';
  resourceKind: 'db';
  healthStatus: 'unhealthy' | 'unknown' | 'mixed';
  serverLength: 'few' | 'none' | 'many' | 'single';
};

const meta: Meta<StoryProps> = {
  title: 'Shared/UnifiedResources/UnhealthyStatusInfo',
  component: UnhealthyStatusInfo,
  argTypes: {
    attemptState: {
      control: { type: 'select' },
      options: ['success', 'processing', 'failed', ''],
    },
    resourceKind: {
      control: { type: 'select' },
      options: ['db'],
    },
    healthStatus: {
      control: { type: 'select' },
      description: 'Specifies the health status of the servers listed',
      options: ['unhealthy', 'mixed', 'unknown'],
    },
    serverLength: {
      control: { type: 'select' },
      options: ['few', 'none', 'many', 'single'],
    },
  },
  // default
  args: {
    attemptState: 'success',
    resourceKind: 'db',
    healthStatus: 'unhealthy',
    serverLength: 'few',
  },
};
export default meta;

export function UnhealthyStatusInfo(props: StoryProps) {
  let attempt: Attempt = { status: props.attemptState };
  if (props.attemptState === 'failed') {
    attempt = { status: 'failed', statusText: 'some kind of error' };
  }

  let resource: UnifiedResourceDefinition;
  let servers: SharedResourceServer[] = [];
  if (props.resourceKind === 'db') {
    resource = {
      kind: 'db',
      type: 'postgres',
      description: 'some database description',
      name: 'testing-database-resource-long-title-name',
      protocol: 'postgres',
      labels: [],
      targetHealth: {
        status: 'unhealthy',
      },
    };

    if (props.healthStatus === 'unhealthy') {
      servers = getDbServers(props, unhealthyDbServers);
    }
    if (props.healthStatus === 'unknown') {
      servers = getDbServers(props, unknownDbServers);
    }
    if (props.healthStatus === 'mixed') {
      servers = getDbServers(props, [
        ...unknownDbServers,
        ...unhealthyDbServers,
      ]);
    }
  }

  return (
    <SlidingSidePanel
      panelWidth={resourceStatusPanelWidth}
      isVisible={true}
      slideFrom="right"
      zIndex={1}
      skipAnimation={false}
    >
      <InfoGuideContainer
        onClose={() => null}
        title={<StatusInfoHeader resource={resource} />}
      >
        <Component
          attempt={attempt}
          resource={resource}
          fetch={() => null}
          servers={servers}
        />
      </InfoGuideContainer>
    </SlidingSidePanel>
  );
}

const loremTxt =
  'Lorem ipsum dolor sit amet consectetur adipisicing elit. \
  Hic facere accusamus vel dolorum sunt, magni incidunt rem \
  quas reiciendis fugiat molestias delectus perspiciatis vero \
  similique minima mollitia accusantium eligendi impedit.';

const unhealthyDbServers: DatabaseServer[] = [
  {
    kind: 'db_server',
    hostId: 'host-id-1',
    hostname: 'hostname-1',
    targetHealth: {
      status: 'unhealthy',
      error: 'unhealthy error reason 1',
      message: 'some unhealthy message 1',
    },
  },
  {
    kind: 'db_server',
    hostId:
      'host-id-long-george-washington-cherry-blossom-apple-banana-orange-chocolate-meow',
    hostname:
      'hostname-long-really-long-like-really-long-longer-pumpkin-pie-halloween',
    targetHealth: { status: 'unhealthy', error: loremTxt },
  },
];

const unknownDbServers: DatabaseServer[] = [
  {
    kind: 'db_server',
    hostId: 'host-id-1',
    hostname: 'hostname-1',
    targetHealth: {
      status: 'unknown',
      error: 'unknown error reason 1',
      message: 'some unknown message 1',
    },
  },
  {
    kind: 'db_server',
    hostId: 'host-id-2',
    hostname: 'hostname-2',
    targetHealth: {
      status: 'unknown',
      error: 'unknown error reason 2',
      message: 'some unknown message 2',
    },
  },
];

function getDbServers(
  props: Pick<StoryProps, 'serverLength'>,
  servers: DatabaseServer[]
) {
  if (props.serverLength === 'many') {
    return [...servers, ...servers, ...servers, ...servers];
  }
  if (props.serverLength === 'few') {
    return servers;
  }
  if (props.serverLength === 'single') {
    return [servers[0]];
  }
}
