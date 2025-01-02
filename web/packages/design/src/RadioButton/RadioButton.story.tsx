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

import styled from 'styled-components';

import { Flex } from '..';
import { RadioButton } from './RadioButton';

export default {
  title: 'Design/RadioButton',
};

export const RadioButtonStory = () => (
  <Flex
    alignItems="start"
    flexDirection="column"
    gap={3}
    bg="levels.surface"
    p={5}
  >
    <Table border={1}>
      <tr>
        <th colSpan={2} />
        <th>Large</th>
        <th>Small</th>
      </tr>
      <tr>
        <th rowSpan={4}>Enabled</th>
        <th>Default</th>
        <td>
          <RadioButton name="en-def-lg" />
          <RadioButton name="en-def-lg" checked />
        </td>
        <td>
          <RadioButton name="en-def-sm" size="small" />
          <RadioButton name="en-def-sm" size="small" checked />
        </td>
      </tr>
      <tr className="teleport-radio-button__force-hover">
        <th>Hover</th>
        <td>
          <RadioButton name="en-hov-lg" />
          <RadioButton name="en-hov-lg" checked />
        </td>
        <td>
          <RadioButton name="en-hov-sm" size="small" />
          <RadioButton name="en-hov-sm" size="small" checked />
        </td>
      </tr>
      <tr className="teleport-radio-button__force-active">
        <th>Active</th>
        <td>
          <RadioButton name="en-act-lg" />
          <RadioButton name="en-act-lg" checked />
        </td>
        <td>
          <RadioButton name="en-act-sm" size="small" />
          <RadioButton name="en-act-sm" size="small" checked />
        </td>
      </tr>
      <tr className="teleport-radio-button__force-focus-visible">
        <th>Focus</th>
        <td>
          <RadioButton name="en-foc-lg" />
          <RadioButton name="en-foc-lg" checked />
        </td>
        <td>
          <RadioButton name="en-foc-sm" size="small" />
          <RadioButton name="en-foc-sm" size="small" checked />
        </td>
      </tr>
      <tr>
        <th rowSpan={4}>Disabled</th>
        <th>Default</th>
        <td>
          <RadioButton name="dis-def-lg" disabled />
          <RadioButton name="dis-def-lg" disabled checked />
        </td>
        <td>
          <RadioButton name="dis-def-sm" size="small" disabled />
          <RadioButton name="dis-def-sm" size="small" disabled checked />
        </td>
      </tr>
      <tr className="teleport-radio-button__force-hover">
        <th>Hover</th>
        <td>
          <RadioButton name="dis-hov-lg" disabled />
          <RadioButton name="dis-hov-lg" disabled checked />
        </td>
        <td>
          <RadioButton name="dis-hov-sm" size="small" disabled />
          <RadioButton name="dis-hov-sm" size="small" disabled checked />
        </td>
      </tr>
      <tr className="teleport-radio-button__force-active">
        <th>Active</th>
        <td>
          <RadioButton name="dis-act-lg" disabled />
          <RadioButton name="dis-act-lg" disabled checked />
        </td>
        <td>
          <RadioButton name="dis-act-sm" size="small" disabled />
          <RadioButton name="dis-act-sm" size="small" disabled checked />
        </td>
      </tr>
    </Table>
    <label>
      <RadioButton size="small" name="uncontrolled" defaultChecked={false} />{' '}
      Uncontrolled radio button, unchecked
    </label>
    <label>
      <RadioButton size="small" name="uncontrolled" defaultChecked={true} />{' '}
      Uncontrolled radio button, checked
    </label>
  </Flex>
);

RadioButtonStory.storyName = 'RadioButton';

const Table = styled.table`
  border-collapse: collapse;
  th,
  td {
    border: ${p => p.theme.borders[1]};
    padding: 10px;
  }
`;
