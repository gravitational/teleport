/*
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

import React from 'react';

import Text, { H1 } from '../Text';

import { H2 } from './Text';

export default {
  title: 'Design/Text',
};

export const FontSizes = () => (
  <>
    <H1>H1 Heading</H1>
    <H2>H2 Heading</H2>
    <Text as="h3" typography="h3">
      h3
    </Text>
    <Text as="h4" typography="h4">
      h4
    </Text>
    <Text as="h5" typography="h5">
      h5
    </Text>
    <Text as="h6" typography="h6">
      h6
    </Text>
  </>
);

export const FontAttributes = () => (
  <>
    <Text regular mb={3}>
      Hello Regular
    </Text>
    <Text bold mb={3}>
      Hello Bold
    </Text>
    <Text caps mb={3}>
      Hello Caps
    </Text>
    <Text italic>Hello italic</Text>
  </>
);

export const FontColor = () => (
  <>
    <Text color="blue">Hello Blue</Text>
    <Text color="green">Hello Green</Text>
  </>
);

export const Alignments = () => (
  <>
    <Text textAlign="left">Hello Left</Text>
    <Text textAlign="center">Hello Center</Text>
    <Text textAlign="right">Hello Right</Text>
  </>
);
