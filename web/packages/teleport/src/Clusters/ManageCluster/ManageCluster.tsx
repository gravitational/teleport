/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import React, { useCallback, useEffect, useState } from 'react';
import { useParams } from 'react-router';
import { Link } from 'react-router-dom';
import styled from 'styled-components';

import { Alert } from 'design/Alert';
import Box, { BoxProps } from 'design/Box';
import Flex from 'design/Flex';
import * as Icons from 'design/Icon';
import { Indicator } from 'design/Indicator';
import { MultiRowBox, Row } from 'design/MultiRowBox';
import { ShimmerBox } from 'design/ShimmerBox';
import Text, { H2 } from 'design/Text';
import { LoadingSkeleton } from 'shared/components/UnifiedResources/shared/LoadingSkeleton';
import { Attempt, useAsync } from 'shared/hooks/useAsync';

import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import cfg from 'teleport/config';
import { useTeleport } from 'teleport/index';
import { useNoMinWidth } from 'teleport/Main';
import { ClusterInfo } from 'teleport/services/clusters';

/**
 * OSS Cluster Management page.
 * @returns JSX.Element
 */
export function ManageCluster() {
  const [cluster, setCluster] = useState<ClusterInfo>(null);
  const ctx = useTeleport();

  const { clusterId } = useParams<{
    clusterId: string;
  }>();

  const [attempt, run] = useAsync(
    useCallback(async () => {
      const res = await ctx.clusterService.fetchClusterDetails(clusterId);
      setCluster(res);
      return res;
    }, [clusterId, ctx.clusterService])
  );

  useEffect(() => {
    if (!attempt.status && clusterId) {
      run();
    }
  }, [attempt.status, run, clusterId]);

  useNoMinWidth();

  return (
    <FeatureBox>
      <ManageClusterHeader clusterId={clusterId} />
      {attempt.status === 'processing' ? (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      ) : (
        <ClusterInformation cluster={cluster} attempt={attempt} />
      )}
    </FeatureBox>
  );
}

export function ManageClusterHeader({ clusterId }: { clusterId: string }) {
  return (
    <FeatureHeader>
      <Flex alignItems="center">
        <Icons.ArrowBack
          as={Link}
          size="large"
          mr={2}
          title="Go Back"
          to={cfg.routes.clusters}
          style={{ cursor: 'pointer', textDecoration: 'none' }}
        />
        <FeatureHeaderTitle>
          <Flex gap="3">
            Manage Clusters
            <Text>/</Text>
            {clusterId}
          </Flex>
        </FeatureHeaderTitle>
      </Flex>
    </FeatureHeader>
  );
}

type ClusterInformationProps = {
  cluster?: ClusterInfo;
  style?: React.CSSProperties;
  attempt: Attempt<ClusterInfo>;
} & BoxProps;

export function ClusterInformation({
  cluster,
  style,
  attempt,
  ...rest
}: ClusterInformationProps) {
  const isLoading = attempt.status === 'processing';
  return (
    <MultiRowBox mb={3} minWidth="180px" style={style} {...rest}>
      <Row>
        <Flex alignItems="center" justifyContent="start">
          <IconBox>
            <Icons.Cluster />
          </IconBox>
          <H2>Cluster Information</H2>
        </Flex>
      </Row>
      <Row>
        {attempt.status === 'error' && <Alert>{attempt.statusText}</Alert>}
        {attempt.status !== 'error' && (
          <>
            <DataItem
              title="Cluster Name"
              data={cluster?.clusterId}
              isLoading={isLoading}
            />
            <DataItem
              title="Teleport Version"
              data={cluster?.authVersion}
              isLoading={isLoading}
            />
            <DataItem
              title="Public Address"
              data={cluster?.publicURL}
              isLoading={isLoading}
            />
            {cfg.tunnelPublicAddress && (
              <DataItem
                title="Public SSH Tunnel"
                data={cfg.tunnelPublicAddress}
                isLoading={isLoading}
              />
            )}
            {cfg.edition === 'ent' &&
              !cfg.isCloud &&
              cluster?.licenseExpiryDateText && (
                <DataItem
                  title="License Expiry"
                  data={cluster?.licenseExpiryDateText}
                  isLoading={isLoading}
                />
              )}
          </>
        )}
      </Row>
    </MultiRowBox>
  );
}

export const IconBox = styled(Box)`
  line-height: 0;
  padding: ${props => props.theme.space[2]}px;
  border-radius: ${props => props.theme.radii[3]}px;
  margin-right: ${props => props.theme.space[3]}px;
  background: ${props => props.theme.colors.interactive.tonal.neutral[0]};
`;

export const DataItem = ({ title = '', data = null, isLoading = false }) => (
  <DataItemFlex>
    <Text typography="body2" bold style={{ width: '136px' }}>
      {title}:
    </Text>
    {isLoading ? (
      <LoadingSkeleton
        count={1}
        Element={
          <ShimmerBox height="20px" width={`${randomRange(30, 200)}px`} />
        }
      />
    ) : (
      <Text typography="body2">{data}</Text>
    )}
  </DataItemFlex>
);

const DataItemFlex = styled(Flex)`
  margin-bottom: ${props => props.theme.space[3]}px;
  align-items: center;
  @media screen and (max-width: ${props => props.theme.breakpoints.mobile}px) {
    flex-direction: column;
    padding-left: ${props => props.theme.space[2]}px;
    align-items: start;
  }
`;

function randomRange(min: number, max: number) {
  return Math.floor(Math.random() * (max - min + 1)) + min;
}
