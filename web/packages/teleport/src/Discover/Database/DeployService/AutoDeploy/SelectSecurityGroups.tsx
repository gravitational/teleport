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

import React, { useEffect, useState } from 'react';

import { Box, ButtonSecondary, Flex, Indicator, Subtitle3, Text } from 'design';
import { FetchStatus } from 'design/DataTable/types';
import * as Icons from 'design/Icon';
import { P, P3 } from 'design/Text/Text';
import { HoverTooltip, IconTooltip } from 'design/Tooltip';
import useAttempt from 'shared/hooks/useAttemptNext';
import { getErrMessage } from 'shared/utils/errorType';
import { pluralize } from 'shared/utils/text';

import { DbMeta } from 'teleport/Discover/useDiscover';
import {
  AwsRdsDatabase,
  integrationService,
  SecurityGroup,
  SecurityGroupRule,
} from 'teleport/services/integrations';

import {
  ButtonBlueText,
  SecurityGroupPicker,
  SecurityGroupWithRecommendation,
} from '../../../Shared';

type TableData = {
  items: SecurityGroupWithRecommendation[];
  nextToken?: string;
  fetchStatus: FetchStatus;
};

export const SelectSecurityGroups = ({
  selectedSecurityGroups,
  setSelectedSecurityGroups,
  dbMeta,
  emitErrorEvent,
  disabled = false,
}: {
  selectedSecurityGroups: string[];
  setSelectedSecurityGroups: React.Dispatch<React.SetStateAction<string[]>>;
  dbMeta: DbMeta;
  emitErrorEvent(err: string): void;
  disabled?: boolean;
}) => {
  const [sgTableData, setSgTableData] = useState<TableData>({
    items: [],
    nextToken: '',
    fetchStatus: 'disabled',
  });

  const { attempt, run } = useAttempt('processing');

  function onSelectSecurityGroup(
    sg: SecurityGroup,
    e: React.ChangeEvent<HTMLInputElement>
  ) {
    if (e.target.checked) {
      return setSelectedSecurityGroups(currentSelectedGroups => [
        ...currentSelectedGroups,
        sg.id,
      ]);
    } else {
      setSelectedSecurityGroups(
        selectedSecurityGroups.filter(id => id !== sg.id)
      );
    }
  }

  async function fetchSecurityGroups({ refresh = false } = {}) {
    run(() =>
      integrationService
        .fetchSecurityGroups(dbMeta.awsIntegration.name, {
          vpcId: dbMeta.awsVpcId,
          region: dbMeta.awsRegion,
          nextToken: sgTableData.nextToken,
        })
        .then(({ securityGroups, nextToken }) => {
          const groupsWithTips = withTips(
            refresh
              ? securityGroups
              : [...sgTableData.items, ...securityGroups],
            dbMeta.selectedAwsRdsDb
          );
          setSgTableData({
            nextToken,
            fetchStatus: nextToken ? '' : 'disabled',
            items: groupsWithTips,
          });
          if (refresh) {
            // Reset so user doesn't unintentionally keep a security group
            // that no longer exists upon refresh.
            setSelectedSecurityGroups([]);
          }
        })
        .catch((err: Error) => {
          const errMsg = getErrMessage(err);
          emitErrorEvent(`fetch security groups error: ${errMsg}`);
          throw err;
        })
    );
  }

  useEffect(() => {
    fetchSecurityGroups();
  }, []);

  return (
    <>
      <Flex alignItems="center" gap={1} mb={2}>
        <Subtitle3>Select ECS Security Groups</Subtitle3>
        <IconTooltip>
          <Text>
            Select ECS security group(s) based on the following requirements:
            <ul>
              <li>
                The selected security group(s) must allow all outbound traffic
                (eg: 0.0.0.0/0)
              </li>
              <li>
                A security group attached to your database(s) must allow inbound
                traffic from a security group you select or from all IPs in the
                subnets you selected
              </li>
            </ul>
          </Text>
        </IconTooltip>
      </Flex>

      <P mb={2}>
        Select ECS security groups to assign to the Fargate service that will be
        running the Teleport Database Service. If you don't select any security
        groups, the default one for the VPC will be used.
      </P>
      {/* TODO(bl-nero): Convert this to an alert box with embedded retry button */}
      {attempt.status === 'failed' && (
        <>
          <Flex my={3}>
            <Icons.Warning size="medium" ml={1} mr={2} color="error.main" />
            <Text>{attempt.statusText}</Text>
          </Flex>
          <ButtonBlueText ml={1} onClick={fetchSecurityGroups}>
            Retry
          </ButtonBlueText>
        </>
      )}
      {attempt.status === 'processing' && (
        <Flex width="904px" justifyContent="center" mt={3}>
          <Indicator />
        </Flex>
      )}
      {attempt.status === 'success' && (
        <Box mt={3}>
          <SecurityGroupPicker
            items={sgTableData.items}
            attempt={attempt}
            fetchNextPage={fetchSecurityGroups}
            fetchStatus={sgTableData.fetchStatus}
            onSelectSecurityGroup={onSelectSecurityGroup}
            selectedSecurityGroups={selectedSecurityGroups}
          />
          <Flex alignItems="center" gap={3} mt={2}>
            <HoverTooltip
              tipContent="Refreshing security groups will reset selections"
              anchorOrigin={{ vertical: 'top', horizontal: 'left' }}
            >
              <ButtonSecondary
                onClick={() => fetchSecurityGroups({ refresh: true })}
                px={2}
                disabled={disabled}
              >
                <Icons.Refresh size="medium" mr={2} /> Refresh
              </ButtonSecondary>
            </HoverTooltip>
            <P3>
              {`${selectedSecurityGroups.length} ${pluralize(selectedSecurityGroups.length, 'security group')} selected`}
            </P3>
          </Flex>
        </Box>
      )}
    </>
  );
};

function withTips(
  securityGroups: SecurityGroup[],
  db?: AwsRdsDatabase
): SecurityGroupWithRecommendation[] {
  // if db is undefined, which is possible in the auto-discovery flow, we can
  // still recommend security groups that allow outbound internet access.
  const trustedGroups = getTrustedSecurityGroups(securityGroups, db);
  return securityGroups.map(group => {
    const isTrusted = trustedGroups.has(group.id);
    const isOutboundAllowed = allowsOutbound(group);
    return {
      ...group,
      tips: getTips(isTrusted, isOutboundAllowed),
      // we recommend when either are true because security group rules are
      // additive, meaning they can select multiple groups for a combined effect
      // of satisfying the database inbound rules and the ECS task outbound
      // rules.
      recommended: isTrusted || isOutboundAllowed,
    };
  });
}

function getTips(isTrusted: boolean, allowsOutbound: boolean): string[] {
  const result: string[] = [];
  if (isTrusted) {
    result.push(
      'The database security group inbound rules allow traffic from this security group'
    );
  }
  if (allowsOutbound) {
    result.push('This security group allows outbound traffic to the internet');
  }
  return result;
}

function allowsOutbound(sg: SecurityGroup): boolean {
  return sg.outboundRules.some(rule => {
    if (!rule) {
      return false;
    }
    const havePorts = allowsOutboundToPorts(rule);
    // this is a heuristic, because an exhaustive analysis is non-trivial.
    return havePorts && rule.cidrs.some(cidr => cidr.cidr === '0.0.0.0/0');
  });
}

function allowsOutboundToPorts(rule: SecurityGroupRule): boolean {
  const publicECRPort = 443;
  const proxyPort = window.location.port;
  if (!proxyPort || proxyPort === '443') {
    // if proxy port is not found or it is 443, then we only check for
    // the HTTPS port.
    return ruleAllowsPort(rule, publicECRPort);
  }
  // otherwise we need to check that the rule allows both proxy and ECR ports.
  return (
    ruleAllowsPort(rule, publicECRPort) &&
    ruleAllowsPort(rule, parseInt(proxyPort, 10))
  );
}

function getTrustedSecurityGroups(
  securityGroups: SecurityGroup[],
  db?: AwsRdsDatabase
): Set<string> {
  const trustedGroups = new Set<string>();
  if (!db || !db.securityGroups || !db.uri) {
    return trustedGroups;
  }

  const dbPort = getPort(db);
  const securityGroupsById = new Map(
    securityGroups.map(group => [group.id, group])
  );
  db.securityGroups.forEach(groupId => {
    const group = securityGroupsById.get(groupId);
    if (!group) {
      return;
    }
    group.inboundRules.forEach(rule => {
      if (!rule.groups.length) {
        // we only care about rules that reference other security groups.
        return;
      }
      if (!ruleAllowsPort(rule, dbPort)) {
        // a group is only trusted if it is trusted for the relevant port.
        return;
      }
      rule.groups.forEach(({ groupId }) => {
        trustedGroups.add(groupId);
      });
    });
  });

  return trustedGroups;
}

function ruleAllowsPort(rule: SecurityGroupRule, port: number): boolean {
  if (rule.ipProtocol === 'all') {
    return true;
  }
  const fromPort = parseInt(rule.fromPort, 10);
  const toPort = parseInt(rule.toPort, 10);
  return port >= fromPort && port <= toPort;
}

function getPort(db: AwsRdsDatabase): number {
  const [, port = '-1'] = db.uri.split(':');
  return parseInt(port, 10);
}
