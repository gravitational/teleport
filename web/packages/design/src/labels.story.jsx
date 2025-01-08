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

import Flex from './Flex';
import Label from './Label';
import LabelState from './LabelState';

export default {
  title: 'Design/Label',
};

export const Labels = () => (
  <>
    <Flex
      height="100px"
      bg="levels.surface"
      justifyContent="center"
      alignItems="center"
    >
      <Label mr={4} kind="primary">
        Primary
      </Label>
      <Label mr={4} kind="secondary">
        Secondary
      </Label>
      <Label mr={4} kind="warning">
        Warning
      </Label>
      <Label kind="danger">Danger</Label>
    </Flex>
    <Flex
      height="100px"
      bg="levels.surface"
      justifyContent="center"
      alignItems="center"
    >
      <LabelState mr="4" kind="success">
        Success
      </LabelState>
      <LabelState mr="4" kind="secondary">
        Secondary
      </LabelState>
      <LabelState mr="4" kind="warning">
        Warning
      </LabelState>
      <LabelState mr="4" kind="danger">
        Danger
      </LabelState>
    </Flex>
    <Flex
      height="100px"
      bg="levels.surface"
      justifyContent="center"
      alignItems="center"
    >
      <LabelState shadow mr="4" kind="success">
        Success
      </LabelState>
      <LabelState shadow mr="4" kind="secondary">
        Secondary
      </LabelState>
      <LabelState shadow mr="4" kind="warning">
        Warning
      </LabelState>
      <LabelState shadow mr="4" kind="danger">
        Danger
      </LabelState>
    </Flex>
  </>
);
