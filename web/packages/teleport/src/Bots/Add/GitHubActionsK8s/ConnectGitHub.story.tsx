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

import { ConnectGitHub } from './ConnectGitHub';
import { GitHubK8sFlowProvider } from './useGitHubK8sFlow';

const meta = {
  title: 'Teleport/Bots/Add/GitHubActions+K8s/ConnectGitHub',
  component: Wrapper,
} satisfies Meta<typeof Wrapper>;

type Story = StoryObj<typeof meta>;

export default meta;

export const Happy: Story = {};

function Wrapper() {
  return (
    <GitHubK8sFlowProvider>
      <Container>
        <ConnectGitHub />
      </Container>
    </GitHubK8sFlowProvider>
  );
}

const Container = styled(Flex)`
  height: 820px;
  overflow: auto;
`;
