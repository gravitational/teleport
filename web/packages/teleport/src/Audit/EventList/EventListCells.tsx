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

import { ButtonBorder } from 'design';
import { Cell } from 'design/DataTable';
import { displayDateTime } from 'design/datetime';

import { Event } from 'teleport/services/audit';

export const ActionCell = props => {
  const { rowIndex, onViewDetails, data } = props;
  const event: Event = data[rowIndex];

  function onClick() {
    onViewDetails(event);
  }
  return (
    <Cell align="right">
      <ButtonBorder size="small" onClick={onClick} width="87px">
        Details
      </ButtonBorder>
    </Cell>
  );
};

export const TimeCell = props => {
  const { rowIndex, data } = props;
  const { time } = data[rowIndex];
  return <Cell style={{ minWidth: '120px' }}>{displayDateTime(time)}</Cell>;
};

export function DescCell(props) {
  const { rowIndex, data } = props;
  const { message } = data[rowIndex];
  return <Cell style={{ wordBreak: 'break-word' }}>{message}</Cell>;
}
