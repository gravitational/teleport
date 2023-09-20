/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import styled from 'styled-components';

import { Text, ButtonSecondary } from 'design';
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
        <Text mb={4} typography="h4">
          {ruleType === 'inbound' ? 'Inbound' : 'Outbound'} Rules for [{sg.name}
          ]
        </Text>
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
