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

import React from 'react';
import styled from 'styled-components';

import { Box, Flex, Indicator, Text } from 'design';
import { AuthenticatorIcon, authenticatorName } from 'design/AuthenticatorIcon';
import { ButtonWarningBorder } from 'design/Button/Button';
import { Cell, DateCell } from 'design/DataTable';
import Table from 'design/DataTable/Table';
import * as Icon from 'design/Icon';
import { MultiRowBox, Row } from 'design/MultiRowBox';
import { IconTooltip } from 'design/Tooltip';
import { Attempt } from 'shared/hooks/useAttemptNext';

import { MfaDevice } from 'teleport/services/mfa';

const ICON_SIZE = 24;

export interface AuthDeviceListProps {
  header: React.ReactNode;
  devices: MfaDevice[];
  onRemove?: (device: MfaDevice) => void;
  attempt: Attempt;
  passkeysEnabled: boolean;
}

/**
 * Renders a table with authentication devices, preceded by a header, all inside
 * a border.
 */
export function AuthDeviceList({
  devices,
  attempt,
  header,
  onRemove,
  passkeysEnabled,
}: AuthDeviceListProps) {
  return (
    <MultiRowBox>
      <Row>{header}</Row>
      {attempt.status == 'processing' && (
        <Row data-testid="device-list-loading">
          <Flex justifyContent="center">
            <Indicator size={40} delay="none" />
          </Flex>
        </Row>
      )}
      {devices.length > 0 && (
        <Row>
          <StyledTable
            columns={[
              {
                key: 'name',
                headerText: 'Device',
                isSortable: true,
                // Sorting by type is no longer a column of its own, so the merged column groups by
                // type before ordering by nickname within each group.
                onSort: (a, b) =>
                  compareLabels(deviceTypeLabel(a), deviceTypeLabel(b)) ||
                  compareLabels(a.name, b.name),
                render: device => {
                  const isWebauthn = device.type === 'webauthn';
                  // Nicknames default to the authenticator's own name, so the type below would often
                  // just repeat the line above it.
                  const deviceType = deviceTypeLabel(device);
                  const content = (
                    <Flex alignItems="center" gap={3}>
                      {/* TOTP and SSO have no vendor mark, but they still reserve the slot so every
                          nickname in the column starts at the same offset. */}
                      <Box width={`${ICON_SIZE}px`} flex="0 0 auto">
                        {isWebauthn && (
                          <AuthenticatorIcon
                            aaguid={device.aaguid}
                            size={ICON_SIZE}
                          />
                        )}
                      </Box>
                      <Flex flexDirection="column">
                        <Text>{device.name}</Text>
                        {!sameLabel(device.name, deviceType) && (
                          <Text typography="body3" color="text.slightlyMuted">
                            {deviceType}
                          </Text>
                        )}
                      </Flex>
                    </Flex>
                  );
                  if (device.usage === 'passwordless' && !passkeysEnabled) {
                    return (
                      <Cell>
                        <Flex alignItems="center" gap={1}>
                          {content}
                          <IconTooltip>
                            This device can be a passkey, but passwordless
                            authentication is disabled
                          </IconTooltip>
                        </Flex>
                      </Cell>
                    );
                  }
                  return <Cell>{content}</Cell>;
                },
              },
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
                render: device => {
                  return (
                    <RemoveCell
                      isSsoDevice={device.type === 'sso'}
                      onRemove={() => onRemove(device)}
                    />
                  );
                },
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

// The authenticator model when the AAGUID identifies one ("YubiKey 5 Series"), otherwise the coarse
// kind the server reports ("Authenticator App").
function deviceTypeLabel(device: MfaDevice): string {
  const name =
    device.type === 'webauthn' ? authenticatorName(device.aaguid) : undefined;

  return name ?? device.description;
}

// Both lines of the column are user-facing labels that can end in a model number, so they collate the
// same way: "YubiKey 9" before "YubiKey 10".
function compareLabels(a: string, b: string): number {
  return a.localeCompare(b, undefined, { numeric: true });
}

function sameLabel(a: string, b: string): boolean {
  return a.trim().toLowerCase() === b.trim().toLowerCase();
}

interface RemoveCellProps {
  onRemove?: () => void;
  isSsoDevice?: boolean;
}

function RemoveCell({ onRemove, isSsoDevice }: RemoveCellProps) {
  return (
    <Cell data-testid="delete-device">
      <ButtonWarningBorder
        disabled={isSsoDevice}
        title={isSsoDevice ? 'SSO device cannot be deleted' : 'Delete'}
        p={2}
        onClick={onRemove}
      >
        <Icon.Trash size="small" />
      </ButtonWarningBorder>
    </Cell>
  );
}

const StyledTable = styled(Table<MfaDevice>)`
  & > tbody > tr > td,
  thead > tr > th {
    font-weight: 300;
    padding-bottom: ${props => props.theme.space[2]}px;
  }
`;
