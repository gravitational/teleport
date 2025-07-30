/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { Meta, StoryObj } from '@storybook/react-vite';
import styled from 'styled-components';

import Flex from 'design/Flex/Flex';

import { TeleportProviderBasic } from 'teleport/mocks/providers';

import { Instance } from './Instance';
import { methods } from './JoinMethodIcon.story';

const meta = {
  title: 'Teleport/Bots/Details/Instance',
  component: Wrapper,
  argTypes: {
    activeAt: {
      control: {
        type: 'date',
      },
    },
    method: {
      control: 'select',
      options: methods,
    },
    version: {
      control: 'select',
      options: ['6.0.0', '5.0.0', '4.4.0', '4.3.999', '3.2.1', '2.0.1'],
    },
    os: {
      control: 'select',
      options: ['windows', 'linux', 'darwin', '--fallback--'].sort(),
    },
  },
} satisfies Meta<typeof Wrapper>;

type Story = StoryObj<typeof meta>;

export default meta;

export const Item: Story = {
  args: {
    id: '686750f5-0f21-4a6f-b151-fa11a603701d',
    activeAt: new Date('2025-07-18T14:54:32Z').getTime(),
    hostname: 'hostname',
    method: 'kubernetes',
    version: '4.4.0',
    os: 'linux',
  },
};

export const ItemWithNoHeartbeatData: Story = {
  args: {
    id: '686750f5-0f21-4a6f-b151-fa11a603701d',
  },
};

type Props = {
  id: Parameters<typeof Instance>[0]['id'];
  version?: Parameters<typeof Instance>[0]['version'];
  hostname?: Parameters<typeof Instance>[0]['hostname'];
  activeAt?: number;
  method?: Parameters<typeof Instance>[0]['method'];
  os?: Parameters<typeof Instance>[0]['os'];
};
function Wrapper(props: Props) {
  return (
    <TeleportProviderBasic>
      <Container>
        <Container400>
          <Instance
            {...props}
            activeAt={
              props.activeAt
                ? new Date(props.activeAt).toISOString()
                : undefined
            }
          />
        </Container400>
      </Container>
    </TeleportProviderBasic>
  );
}

const Container = styled(Flex)`
  align-items: center;
  justify-content: center;
  padding: 32px;
`;

const Container400 = styled.div`
  width: 400px;
`;
