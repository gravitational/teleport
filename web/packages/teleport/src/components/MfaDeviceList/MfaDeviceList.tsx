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

import { ButtonBorder, Text } from 'design';
import Table, { Cell } from 'design/DataTable';
import { displayDate } from 'design/datetime';
import { dateMatcher } from 'design/utils/match';

import { MfaDevice } from 'teleport/services/mfa/types';

export default function MfaDeviceList({
  devices = [],
  remove,
  mostRecentDevice,
  mfaDisabled = false,
  isSearchable = false,
  style,
}: Props) {
  return (
    <StyledTable
      data={devices}
      style={style}
      columns={[
        {
          key: 'description',
          headerText: 'Type',
        },
        {
          key: 'name',
          headerText: 'Device Name',
          render: renderNameCell,
        },
        {
          key: 'registeredDate',
          headerText: 'Registered',
          isSortable: true,
          render: ({ registeredDate }) => (
            <Cell>{displayDate(registeredDate)}</Cell>
          ),
        },
        {
          key: 'lastUsedDate',
          headerText: 'Last Used',
          isSortable: true,
          render: ({ lastUsedDate }) => (
            <Cell>{displayDate(lastUsedDate)}</Cell>
          ),
        },
        {
          altKey: 'remove-btn',
          render: device =>
            renderRemoveCell(device, remove, mostRecentDevice, mfaDisabled),
        },
      ]}
      emptyText="No Devices Found"
      isSearchable={isSearchable}
      initialSort={{
        key: 'registeredDate',
        dir: 'DESC',
      }}
      customSearchMatchers={[dateMatcher(['registeredDate', 'lastUsedDate'])]}
    />
  );
}

const renderNameCell = ({ name }: MfaDevice) => {
  return (
    <Cell title={name}>
      <Text
        style={{
          maxWidth: '96px',
          whiteSpace: 'nowrap',
        }}
      >
        {name}
      </Text>
    </Cell>
  );
};

const renderRemoveCell = (
  { id, name }: MfaDevice,
  remove: ({ id, name }) => void,
  mostRecentDevice: MfaDevice,
  mfaDisabled: boolean
) => {
  if (id === mostRecentDevice?.id) {
    return <Cell align="right"></Cell>;
  }

  return (
    <Cell align="right">
      <ButtonBorder
        size="small"
        onClick={() => remove({ id, name })}
        disabled={mfaDisabled}
        title={mfaDisabled ? 'Two-factor authentication is disabled' : ''}
      >
        Remove
      </ButtonBorder>
    </Cell>
  );
};

type Props = {
  devices: MfaDevice[];
  remove({ id, name }: { id: string; name: string }): void;
  mostRecentDevice?: MfaDevice;
  mfaDisabled?: boolean;
  isSearchable?: boolean;
  [key: string]: any;
};

const StyledTable = styled(Table)`
  & > tbody > tr {
    td {
      vertical-align: middle;
      height: 32px;
    }
  }
  ${props =>
    !props.isSearchable &&
    `border-radius: 8px; overflow: hidden; box-shadow: ${props.theme.boxShadow[0]};   
    & > tbody {
      background: ${props.theme.colors.levels.elevated};
    }
    & > thead > tr > th {
      background: ${props.theme.colors.spotBackground[1]};
  }
  `}
` as typeof Table;
