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
import { CheckboxInput } from './Checkbox';

export default {
  title: 'Design/Checkbox',
};

export const Checkbox = () => (
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
          <CheckboxInput type="checkbox" />
          <CheckboxInput checked />
        </td>
        <td>
          <CheckboxInput size="small" />
          <CheckboxInput size="small" checked />
        </td>
      </tr>
      <tr className="teleport-checkbox__force-hover">
        <th>Hover</th>
        <td>
          <CheckboxInput type="checkbox" />
          <CheckboxInput checked />
        </td>
        <td>
          <CheckboxInput size="small" />
          <CheckboxInput size="small" checked />
        </td>
      </tr>
      <tr className="teleport-checkbox__force-active">
        <th>Active</th>
        <td>
          <CheckboxInput type="checkbox" />
          <CheckboxInput checked />
        </td>
        <td>
          <CheckboxInput size="small" />
          <CheckboxInput size="small" checked />
        </td>
      </tr>
      <tr className="teleport-checkbox__force-focus-visible">
        <th>Focus</th>
        <td>
          <CheckboxInput type="checkbox" />
          <CheckboxInput checked />
        </td>
        <td>
          <CheckboxInput size="small" />
          <CheckboxInput size="small" checked />
        </td>
      </tr>
      <tr>
        <th rowSpan={4}>Disabled</th>
        <th>Default</th>
        <td>
          <CheckboxInput disabled />
          <CheckboxInput disabled checked />
        </td>
        <td>
          <CheckboxInput size="small" disabled />
          <CheckboxInput size="small" disabled checked />
        </td>
      </tr>
      <tr className="teleport-checkbox__force-hover">
        <th>Hover</th>
        <td>
          <CheckboxInput disabled />
          <CheckboxInput disabled checked />
        </td>
        <td>
          <CheckboxInput size="small" disabled />
          <CheckboxInput size="small" disabled checked />
        </td>
      </tr>
      <tr className="teleport-checkbox__force-active">
        <th>Active</th>
        <td>
          <CheckboxInput disabled />
          <CheckboxInput disabled checked />
        </td>
        <td>
          <CheckboxInput size="small" disabled />
          <CheckboxInput size="small" disabled checked />
        </td>
      </tr>
    </Table>
    <label>
      <CheckboxInput size="small" defaultChecked={false} /> Uncontrolled
      checkbox, unchecked
    </label>
    <label>
      <CheckboxInput size="small" defaultChecked={true} /> Uncontrolled
      checkbox, checked
    </label>
  </Flex>
);

const Table = styled.table`
  border-collapse: collapse;
  th,
  td {
    border: ${p => p.theme.borders[1]};
    padding: 10px;
  }
`;
