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
import { Danger } from 'design/Alert';
import { CheckboxInput } from 'design/Checkbox';
import Table, { Cell } from 'design/DataTable';
import { FetchStatus } from 'design/DataTable/types';
import { IconTooltip } from 'design/Tooltip';
import { Attempt } from 'shared/hooks/useAttemptNext';

import {
  SecurityGroup,
  SecurityGroupRule,
} from 'teleport/services/integrations';

import { SecurityGroupRulesDialog } from './SecurityGroupRulesDialog';

export type SecurityGroupWithRecommendation = SecurityGroup & {
  recommended?: boolean;
  tips?: string[];
};

type Props = {
  attempt: Attempt;
  items: SecurityGroupWithRecommendation[];
  fetchStatus: FetchStatus;
  fetchNextPage(): void;
  onSelectSecurityGroup: (
    sg: SecurityGroup,
    e: React.ChangeEvent<HTMLInputElement>
  ) => void;
  selectedSecurityGroups: string[];
};

export type ViewRulesSelection = {
  name: string;
  rules: ExpandedSecurityGroupRule[];
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
    setViewRulesSelection(undefined);
  }

  if (attempt.status === 'failed') {
    return <Danger>{attempt.statusText}</Danger>;
  }

  const hasRecommendSGs = items.some(item => item.recommended);

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
              const rules = expandSecurityGroupRules(sg.inboundRules);
              return (
                <Cell>
                  <Link
                    style={{ cursor: 'pointer' }}
                    onClick={() =>
                      setViewRulesSelection({
                        name: sg.name,
                        rules: rules,
                        ruleType: 'inbound',
                      })
                    }
                  >
                    View ({rules.length})
                  </Link>
                </Cell>
              );
            },
          },
          {
            altKey: 'outboundRules',
            headerText: 'Outbound Rules',
            render: sg => {
              const rules = expandSecurityGroupRules(sg.outboundRules);
              return (
                <Cell>
                  <Link
                    style={{ cursor: 'pointer' }}
                    onClick={() =>
                      setViewRulesSelection({
                        name: sg.name,
                        rules: rules,
                        ruleType: 'outbound',
                      })
                    }
                  >
                    View ({rules.length})
                  </Link>
                </Cell>
              );
            },
          },
          ...(hasRecommendSGs
            ? [
                {
                  altKey: 'tooltip',
                  headerText: '',
                  render: (sg: SecurityGroupWithRecommendation) => {
                    if (sg.recommended && sg.tips?.length) {
                      return (
                        <Cell>
                          <IconTooltip>
                            <ul>
                              {sg.tips.map((tip, index) => (
                                <li key={index}>{tip}</li>
                              ))}
                            </ul>
                          </IconTooltip>
                        </Cell>
                      );
                    }
                    return <Cell />;
                  },
                },
              ]
            : []),
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
          data-testid={item.id}
        />
      </Flex>
    </Cell>
  );
}

type ExpandedSecurityGroupRule = {
  // IPProtocol is the protocol used to describe the rule.
  ipProtocol: string;
  // FromPort is the inclusive start of the Port range for the Rule.
  fromPort: string;
  // ToPort is the inclusive end of the Port range for the Rule.
  toPort: string;
  // Source is IP range, security group ID, or prefix list that the rule applies to.
  source: string;
  // Description contains a small text describing the source.
  description: string;
};

// expandSecurityGroupRule takes a security group rule in the compact form that
// AWS API returns, wherein rules are grouped by port range, and expands the
// rule into a list of rules that is not grouped by port range.
// This is the same display format that the AWS console uses when you view a
// security group's rules.
function expandSecurityGroupRule(
  rule: SecurityGroupRule
): ExpandedSecurityGroupRule[] {
  return [
    ...rule.cidrs.map(cidr => ({
      source: cidr.cidr,
      description: cidr.description,
    })),
    ...rule.groups.map(group => ({
      source: group.groupId,
      description: group.description,
    })),
  ].map(entry => ({
    ipProtocol: rule.ipProtocol,
    fromPort: rule.fromPort,
    toPort: rule.toPort,
    source: entry.source,
    description: entry.description,
  }));
}

function expandSecurityGroupRules(
  rules: SecurityGroupRule[]
): ExpandedSecurityGroupRule[] {
  return rules.flatMap(rule => expandSecurityGroupRule(rule));
}
