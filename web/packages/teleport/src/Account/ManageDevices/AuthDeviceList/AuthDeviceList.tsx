/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { Cell, DateCell } from 'design/DataTable';
import Table from 'design/DataTable/Table';
import React from 'react';

import styled from 'styled-components';
import { MultiRowBox, Row } from 'design/MultiRowBox';
import * as Icon from 'design/Icon';
import { ButtonWarningBorder } from 'design/Button/Button';

import { MfaDevice } from 'teleport/services/mfa';

export interface AuthDeviceListProps {
  header: React.ReactNode;
  deviceTypeColumnName: string;
  devices: MfaDevice[];
  onRemove?: (device: MfaDevice) => void;
}

/**
 * Renders a table with authentication devices, preceded by a header, all inside
 * a border.
 */
export function AuthDeviceList({
  devices,
  header,
  deviceTypeColumnName,
  onRemove,
}: AuthDeviceListProps) {
  return (
    <MultiRowBox>
      <Row>{header}</Row>
      {devices.length > 0 && (
        <Row>
          <StyledTable<MfaDevice>
            columns={[
              {
                key: 'description',
                headerText: deviceTypeColumnName,
                isSortable: true,
              },
              { key: 'name', headerText: 'Nickname', isSortable: true },
              {
                key: 'registeredDate',
                headerText: 'Added',
                isSortable: true,
                render: device => <DateCell data={device.registeredDate} />,
              },
              {
                key: 'lastUsedDate',
                headerText: 'Last Used',
                isSortable: true,
                render: device => <DateCell data={device.lastUsedDate} />,
              },
              {
                altKey: 'remove-btn',
                headerText: 'Actions',
                render: device => (
                  <RemoveCell onRemove={() => onRemove(device)} />
                ),
              },
            ]}
            data={devices}
            emptyText=""
            isSearchable={false}
            initialSort={{
              key: 'registeredDate',
              dir: 'DESC',
            }}
          />
        </Row>
      )}
    </MultiRowBox>
  );
}

interface RemoveCellProps {
  onRemove?: () => void;
}

function RemoveCell({ onRemove }: RemoveCellProps) {
  return (
    <Cell>
      <ButtonWarningBorder p={2} onClick={onRemove}>
        <Icon.Trash size="small" />
      </ButtonWarningBorder>
    </Cell>
  );
}

const StyledTable = styled(Table)`
  & > tbody > tr > td,
  thead > tr > th {
    font-weight: 300;
    padding-bottom: ${props => props.theme.space[2]}px;
  }
`;
