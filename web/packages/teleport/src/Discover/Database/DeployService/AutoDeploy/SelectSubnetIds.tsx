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

import {
  Box,
  ButtonSecondary,
  Flex,
  Indicator,
  P3,
  Subtitle3,
  Text,
} from 'design';
import { FetchStatus } from 'design/DataTable/types';
import * as Icons from 'design/Icon';
import { HoverTooltip, IconTooltip } from 'design/Tooltip';
import useAttempt from 'shared/hooks/useAttemptNext';
import { getErrMessage } from 'shared/utils/errorType';
import { pluralize } from 'shared/utils/text';

import { SubnetIdPicker } from 'teleport/Discover/Shared/SubnetIdPicker';
import { DbMeta } from 'teleport/Discover/useDiscover';
import { integrationService, Subnet } from 'teleport/services/integrations';
import useTeleport from 'teleport/useTeleport';

import { ButtonBlueText } from '../../../Shared';

type TableData = {
  items: Subnet[];
  nextToken?: string;
  fetchStatus: FetchStatus;
};

export function SelectSubnetIds({
  selectedSubnetIds,
  onSelectedSubnetIds,
  dbMeta,
  emitErrorEvent,
  disabled = false,
}: {
  selectedSubnetIds: string[];
  onSelectedSubnetIds: React.Dispatch<React.SetStateAction<string[]>>;
  dbMeta: DbMeta;
  emitErrorEvent(err: string): void;
  disabled?: boolean;
}) {
  const ctx = useTeleport();
  const clusterId = ctx.storeUser.getClusterId();
  const [tableData, setTableData] = useState<TableData>({
    items: [],
    nextToken: '',
    fetchStatus: 'disabled',
  });

  const { attempt, run } = useAttempt('processing');

  function handleSelectSubnet(
    subnet: Subnet,
    e: React.ChangeEvent<HTMLInputElement>
  ) {
    if (e.target.checked) {
      return onSelectedSubnetIds(currentSelectedGroups => [
        ...currentSelectedGroups,
        subnet.id,
      ]);
    } else {
      onSelectedSubnetIds(selectedSubnetIds.filter(id => id !== subnet.id));
    }
  }

  async function fetchSubnets({ refresh = false } = {}) {
    run(() =>
      integrationService
        .fetchAwsSubnets(dbMeta.awsIntegration.name, clusterId, {
          vpcId: dbMeta.awsVpcId,
          region: dbMeta.awsRegion,
          nextToken: tableData.nextToken,
        })
        .then(({ subnets, nextToken }) => {
          const combinedSubnets = [...tableData.items, ...subnets];
          setTableData({
            nextToken,
            fetchStatus: nextToken ? '' : 'disabled',
            items: refresh ? subnets : combinedSubnets,
          });
          if (refresh) {
            // Reset so user doesn't unintentionally keep a subnet
            // that no longer exists upon refresh.
            onSelectedSubnetIds([]);
          }
        })
        .catch((err: Error) => {
          const errMsg = getErrMessage(err);
          emitErrorEvent(`fetch subnets error: ${errMsg}`);
          throw err;
        })
    );
  }

  useEffect(() => {
    fetchSubnets();
  }, []);

  return (
    <>
      <Flex alignItems="center" gap={1} mb={2}>
        <Subtitle3>Select ECS Subnets</Subtitle3>
        <IconTooltip>
          <Text>
            A subnet has an outbound internet route if it has a route to an
            internet gateway or a NAT gateway in a public subnet.
          </Text>
        </IconTooltip>
      </Flex>

      <Text mb={2}>
        Select ECS subnets to assign to the Fargate service that will be running
        the Teleport Database Service. All of the subnets you select must have
        an outbound internet route and a local route to the database subnets.
      </Text>
      {attempt.status === 'failed' && (
        <>
          <Flex my={3}>
            <Icons.Warning size="medium" ml={1} mr={2} color="error.main" />
            <Text>{attempt.statusText}</Text>
          </Flex>
          <ButtonBlueText ml={1} onClick={fetchSubnets}>
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
          <SubnetIdPicker
            region={dbMeta.awsRegion}
            subnets={tableData.items}
            attempt={attempt}
            fetchNextPage={fetchSubnets}
            fetchStatus={tableData.fetchStatus}
            onSelectSubnet={handleSelectSubnet}
            selectedSubnets={selectedSubnetIds}
          />
          <Flex alignItems="center" gap={3} mt={2}>
            <HoverTooltip
              tipContent="Refreshing subnets will reset selections"
              placement="top-start"
            >
              <ButtonSecondary
                onClick={() => fetchSubnets({ refresh: true })}
                px={2}
                disabled={disabled}
              >
                <Icons.Refresh size="medium" mr={2} /> Refresh
              </ButtonSecondary>
            </HoverTooltip>
            <P3>
              {`${selectedSubnetIds.length} ${pluralize(selectedSubnetIds.length, 'subnet')} selected`}
            </P3>
          </Flex>
        </Box>
      )}
    </>
  );
}
