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

import React, { useState, useEffect } from 'react';

import { Text, Flex, Box, Indicator, ButtonSecondary, Subtitle3 } from 'design';
import * as Icons from 'design/Icon';
import { FetchStatus } from 'design/DataTable/types';
import { HoverTooltip, ToolTipInfo } from 'shared/components/ToolTip';
import useAttempt from 'shared/hooks/useAttemptNext';
import { getErrMessage } from 'shared/utils/errorType';
import { pluralize } from 'shared/utils/text';
import { P, P3 } from 'design/Text/Text';

import {
  integrationService,
  SecurityGroup,
} from 'teleport/services/integrations';
import { DbMeta } from 'teleport/Discover/useDiscover';

import { SecurityGroupPicker, ButtonBlueText } from '../../../Shared';

type TableData = {
  items: SecurityGroup[];
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
          const combinedSgs = [...sgTableData.items, ...securityGroups];
          setSgTableData({
            nextToken,
            fetchStatus: nextToken ? '' : 'disabled',
            items: refresh ? securityGroups : combinedSgs,
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
        <Subtitle3>Select Security Groups</Subtitle3>
        <ToolTipInfo>
          <Text>
            Select security group(s) based on the following requirements:
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
        </ToolTipInfo>
      </Flex>

      <P mb={2}>
        Select security groups to assign to the Fargate service that will be
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
