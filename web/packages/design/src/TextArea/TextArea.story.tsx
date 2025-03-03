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

import Box from 'design/Box';
import Flex from 'design/Flex';
import { H2 } from 'design/Text/Text';

import { TextArea } from './TextArea';

export default {
  title: 'Design/TextArea',
};

export const TextAreas = () => (
  <>
    <TextArea mb={4} placeholder="Enter Some long text" />
    <TextArea mb={4} hasError={true} defaultValue="This field has an error" />
    <TextArea mb={4} readOnly defaultValue="This field is read-only" />
    <TextArea mb={4} disabled defaultValue="This field is disabled" />
    <TextArea mb={4} disabled placeholder="Disabled field with a placeholder" />
    <TextArea
      mb={4}
      resizable={true}
      defaultValue="This field is resizable vertically"
    />
    <H2 mb={2}>Sizes</H2>
    <Flex gap={4} mb={4}>
      <Box flex="1">
        <TextArea size="large" defaultValue="large" />
      </Box>
      <Box flex="1">
        <TextArea size="medium" defaultValue="medium" />
      </Box>
      <Box flex="1">
        <TextArea size="small" defaultValue="small" />
      </Box>
    </Flex>
    <Flex gap={4}>
      <Box flex="1">
        <TextArea hasError size="large" defaultValue="large" />
      </Box>
      <Box flex="1">
        <TextArea hasError size="medium" defaultValue="medium" />
      </Box>
      <Box flex="1">
        <TextArea hasError size="small" defaultValue="small" />
      </Box>
    </Flex>
  </>
);
