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
import { IconProps } from 'design/Icon/Icon';

import { JoinMethodIcon } from './JoinMethodIcon';

const meta = {
  title: 'Teleport/Bots/Details/JoinMethodIcon',
  component: Wrapper,
  excludeStories: ['methods'],
  argTypes: {
    size: {
      control: 'select',
      options: ['fill', 'small', 'medium', 'large', 'extra-large'],
    },
  },
} satisfies Meta<typeof Wrapper>;

type Story = StoryObj<typeof meta>;

export default meta;

export const AllMethods: Story = {
  args: {
    size: 'extra-large',
  },
};

function Wrapper(props: { size: IconProps['size'] }) {
  return (
    <Container>
      {methods.map(m => (
        <Flex key={m} flexDirection={'column'} alignItems={'center'} gap={2}>
          <IconContainer>
            <JoinMethodIcon
              method={m}
              size={props.size}
              color={hasIcon.includes(m) ? undefined : 'red'}
            />
          </IconContainer>
          {m}
        </Flex>
      ))}
    </Container>
  );
}

const Container = styled(Flex)`
  display: grid;
  grid-template-columns: repeat(5, auto);
  gap: ${props => props.theme.space[4]}px;
  align-items: center;
  justify-content: center;
`;

const IconContainer = styled(Flex)`
  align-items: center;
  justify-content: center;
  border: 1px dashed ${p => p.theme.colors.text.main};
`;

export const methods = [
  'token',
  'ec2',
  'iam',
  'github',
  'circleci',
  'kubernetes',
  'azure',
  'gitlab',
  'gcp',
  'spacelift',
  'tpm',
  'terraform_cloud',
  'bitbucket',
  'oracle',
  'azure_devops',
  'bound_keypair',
  '--fallback--',
].sort();

const hasIcon = [
  'token',
  'ec2',
  'iam',
  'github',
  'kubernetes',
  'tpm',
  'bound_keypair',
  '--fallback--',
];
