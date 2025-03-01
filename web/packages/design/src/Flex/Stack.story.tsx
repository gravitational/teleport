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

import styled from 'styled-components';

import Box from 'design/Box';
import { ButtonPrimary } from 'design/Button';
import { P } from 'design/Text/Text';

import { Stack } from './Flex';

export default {
  title: 'Design/Flex/Stack',
};

export const Basic = () => (
  <Stack gap={6}>
    <Square bg="pink" />

    <Stack gap={3}>
      {/* If no gap prop is given, a default gap of 1 is used. */}
      <Stack>
        <Square bg="green" />
        <ButtonPrimary>Foo</ButtonPrimary>
      </Stack>
      <Stack>
        <Square bg="green" />
        <ButtonPrimary>Bar</ButtonPrimary>
      </Stack>
      <Stack>
        <Square bg="green" />
        <ButtonPrimary>Baz</ButtonPrimary>
      </Stack>
    </Stack>

    <Square bg="yellow" />
  </Stack>
);

export const MarginAuto = () => (
  <Stack gap={6} height="90vh">
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

const Square = styled(Box).attrs({ width: '150px', height: '150px' })``;
const SmallSquare = styled(Box).attrs({ width: '50px', height: '50px' })``;
