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

import React, { useState } from 'react';

import { Flex, Link } from 'design';
import Table, { Cell } from 'design/DataTable';
import { Danger } from 'design/Alert';
import { CheckboxInput } from 'design/Checkbox';
import { FetchStatus } from 'design/DataTable/types';

import { Attempt } from 'shared/hooks/useAttemptNext';

import { SecurityGroup } from 'teleport/services/integrations';

import { SecurityGroupRulesDialog } from './SecurityGroupRulesDialog';

type Props = {
  attempt: Attempt;
  items: SecurityGroup[];
  fetchStatus: FetchStatus;
  fetchNextPage(): void;
  onSelectSecurityGroup: (
    sg: SecurityGroup,
    e: React.ChangeEvent<HTMLInputElement>
  ) => void;
  selectedSecurityGroups: string[];
};

export type ViewRulesSelection = {
  sg: SecurityGroup;
  ruleType: 'inbound' | 'outbound';
};

export const SecurityGroupPicker = ({
  attempt,
  items = [],
  fetchStatus = '',
  fetchNextPage,
  onSelectSecurityGroup,
  selectedSecurityGroups,
}: Props) => {
  const [viewRulesSelection, setViewRulesSelection] =
    useState<ViewRulesSelection>();

  function onCloseRulesDialog() {
    setViewRulesSelection(null);
  }

  if (attempt.status === 'failed') {
    return <Danger>{attempt.statusText}</Danger>;
  }

  return (
    <>
      <Table
        data={items}
        columns={[
          {
            altKey: 'checkbox-select',
            headerText: 'Select',
            render: item => {
              const isChecked = selectedSecurityGroups.includes(item.id);
              return (
                <CheckboxCell
                  item={item}
                  key={item.id}
                  isChecked={isChecked}
                  onChange={onSelectSecurityGroup}
                />
              );
            },
          },
          {
            key: 'name',
            headerText: 'Name',
          },
          {
            key: 'id',
            headerText: 'ID',
          },
          {
            key: 'description',
            headerText: 'Description',
          },
          {
            altKey: 'inboundRules',
            headerText: 'Inbound Rules',
            render: sg => {
              return (
                <Cell>
                  <Link
                    style={{ cursor: 'pointer' }}
                    onClick={() =>
                      setViewRulesSelection({ sg, ruleType: 'inbound' })
                    }
                  >
                    View ({sg.inboundRules.length})
                  </Link>
                </Cell>
              );
            },
          },
          {
            altKey: 'outboundRules',
            headerText: 'Outbound Rules',
            render: sg => {
              return (
                <Cell>
                  <Link
                    style={{ cursor: 'pointer' }}
                    onClick={() =>
                      setViewRulesSelection({ sg, ruleType: 'outbound' })
                    }
                  >
                    View ({sg.outboundRules.length})
                  </Link>
                </Cell>
              );
            },
          },
        ]}
        emptyText="No Security Groups Found"
        pagination={{ pageSize: 5 }}
        fetching={{ onFetchMore: fetchNextPage, fetchStatus }}
        isSearchable
      />
      {viewRulesSelection && (
        <SecurityGroupRulesDialog
          viewRulesSelection={viewRulesSelection}
          onClose={onCloseRulesDialog}
        />
      )}
    </>
  );
};

function CheckboxCell({
  item,
  isChecked,
  onChange,
}: {
  item: SecurityGroup;
  isChecked: boolean;
  onChange(
    selectedItem: SecurityGroup,
    e: React.ChangeEvent<HTMLInputElement>
  ): void;
}) {
  return (
    <Cell width="20px">
      <Flex alignItems="center" my={2} justifyContent="center">
        <CheckboxInput
          id={item.id}
          onChange={e => {
            onChange(item, e);
          }}
          checked={isChecked}
        />
      </Flex>
    </Cell>
  );
}
