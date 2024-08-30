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

import React from 'react';
import styled from 'styled-components';

import { ButtonSecondary, H2 } from 'design';
import Table, { Cell } from 'design/DataTable';
import Dialog, { DialogContent, DialogFooter } from 'design/DialogConfirmation';

import { ViewRulesSelection } from './SecurityGroupPicker';

export function SecurityGroupRulesDialog({
  viewRulesSelection,
  onClose,
}: {
  viewRulesSelection: ViewRulesSelection;
  onClose: () => void;
}) {
  const { ruleType, sg } = viewRulesSelection;
  const data = ruleType === 'inbound' ? sg.inboundRules : sg.outboundRules;

  return (
    <Dialog disableEscapeKeyDown={false} open={true}>
      <DialogContent
        width="600px"
        alignItems="center"
        mb={0}
        textAlign="center"
      >
        <H2 mb={4}>
          {ruleType === 'inbound' ? 'Inbound' : 'Outbound'} Rules for [{sg.name}
          ]
        </H2>
        <StyledTable
          data={data}
          columns={[
            {
              key: 'ipProtocol',
              headerText: 'Type',
            },
            {
              altKey: 'portRange',
              headerText: 'Port Range',
              render: ({ fromPort, toPort }) => {
                // If they are the same, only show one number.
                const portRange =
                  fromPort === toPort ? fromPort : `${fromPort} - ${toPort}`;
                return <Cell>{portRange}</Cell>;
              },
            },
            {
              altKey: 'source',
              headerText: 'Source',
              render: ({ cidrs }) => {
                // The AWS API returns an array, however it appears it's not actually possible to have multiple CIDR's for a single rule.
                // As a fallback we just display the first one.
                const cidr = cidrs[0];
                if (cidr) {
                  return <Cell>{cidr.cidr}</Cell>;
                }
                return null;
              },
            },
            {
              altKey: 'description',
              headerText: 'Description',
              render: ({ cidrs }) => {
                const cidr = cidrs[0];
                if (cidr) {
                  return <Cell>{cidr.description}</Cell>;
                }
                return null;
              },
            },
          ]}
          emptyText="No Rules Found"
        />
      </DialogContent>
      <DialogFooter
        css={`
          display: flex;
          justify-content: flex-end;
        `}
      >
        <ButtonSecondary mt={3} onClick={onClose}>
          Close
        </ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}

const StyledTable = styled(Table)`
  & > tbody > tr > td {
    vertical-align: middle;
    text-align: left;
  }

  & > thead > tr > th {
    background: ${props => props.theme.colors.spotBackground[1]};
  }

  border-radius: 8px;
  box-shadow: ${props => props.theme.boxShadow[0]};
  overflow: hidden;
` as typeof Table;
