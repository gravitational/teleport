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

import { Meta } from '@storybook/react';
import styled from 'styled-components';

import Box from 'design/Box';
import { ButtonPrimary } from 'design/Button';
import { P, P2 } from 'design/Text/Text';

import { Stack } from './Flex';

type StoryProps = {
  fullWidth: boolean;
};

const meta: Meta<StoryProps> = {
  title: 'Design/Flex/Stack',
  args: {
    fullWidth: false,
  },
};
export default meta;

export const Basic = ({ fullWidth }: StoryProps) => (
  <Stack gap={6} fullWidth={fullWidth}>
    <Square bg="pink" />

    <Stack gap={3} fullWidth={fullWidth}>
      {/* If no gap prop is given, a default gap of 1 is used. */}
      <Stack>
        <Square bg="green" />
        <ButtonPrimary>Foo</ButtonPrimary>
      </Stack>
      <Stack fullWidth={fullWidth}>
        <Square bg="green">
          {fullWidth && (
            <Para>
              Only the middle one among these three sibling Stacks is given{' '}
              <code>fullWidth</code>. This demonstrates that{' '}
              <code>fullWidth</code> can be used for specific child Stacks
              within multiple levels of nested Stacks.
            </Para>
          )}
        </Square>
        <ButtonPrimary>Bar</ButtonPrimary>
      </Stack>
      <Stack width="100%">
        <Square bg="green" width="100%">
          {fullWidth && (
            <Para>
              With <code>fullWidth</code>, all immediate children of a Stack
              have the width set to 100%. If only specific children are supposed
              to have 100% width, the width can be set manually, like in this
              last stack. Notice how the button doesn't span full width.
            </Para>
          )}
        </Square>
        <ButtonPrimary>Baz</ButtonPrimary>
      </Stack>
    </Stack>

    <Square bg="yellow" />
  </Stack>
);

export const MarginAuto = ({ fullWidth }: StoryProps) => (
  <Stack gap={6} height="90vh" fullWidth={fullWidth}>
    <P>
      <code>margin-top: auto</code> can be used to automatically align elements
      after a certain child to the end of the stack.
    </P>
    <SmallSquare bg="pink" />
    <SmallSquare bg="green" />
    <SmallSquare bg="brown" />
    <SmallSquare bg="yellow" marginTop="auto" />
    <SmallSquare bg="orange" />
  </Stack>
);

const Square = styled(Box)``;
Square.defaultProps = { width: '150px', height: '150px', p: 2 };
const SmallSquare = styled(Box).attrs({ width: '50px', height: '50px' })``;
const Para = styled(P2)`
  max-width: 60ch;
`;
