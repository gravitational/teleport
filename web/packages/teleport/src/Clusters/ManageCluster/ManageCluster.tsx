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

import { MultiRowBox, Row } from 'design/MultiRowBox';
import Flex from 'design/Flex';
import * as Icons from 'design/Icon';
import Text, { H2 } from 'design/Text';

import Box, { BoxProps } from 'design/Box';

import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import { useTeleport } from 'teleport/index';
import cfg from 'teleport/config';
import { useNoMinWidth } from 'teleport/Main';
import { Cluster } from 'teleport/services/clusters';

export function ManageCluster() {
  // TODO: use cluster ID from path?
  // const { clusterId } = useParams<{
  //   clusterId: string;
  // }>();
  const ctx = useTeleport();
  const cluster = ctx.storeUser.state.cluster;

  useNoMinWidth();

  return (
    <FeatureBox>
      <FeatureHeader alignItems="center">
        <FeatureHeaderTitle>
          {/* todo breadcrumbs/header */}
          Manage Clusters / {cluster.clusterId}
        </FeatureHeaderTitle>
      </FeatureHeader>
      <ClusterInformation cluster={cluster} />
    </FeatureBox>
  );
}

type ClusterInformationProps = {
  cluster: Cluster;
  style?: React.CSSProperties;
} & BoxProps;
export function ClusterInformation({
  cluster,
  style,
  ...rest
}: ClusterInformationProps) {
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
        <DataItem title="Cluster Name" data={cluster.clusterId} />
        <DataItem title="Teleport Version" data={cluster.authVersion} />
        <DataItem title="Public Address" data={cluster.publicURL} />
        {cfg.tunnelPublicAddress && (
          <DataItem title="Public SSH Tunnel" data={cfg.tunnelPublicAddress} />
        )}
        {cfg.edition === 'ent' &&
          !cfg.isCloud &&
          cluster.licenseExpiryDateText && (
            <DataItem
              title="License Expiry"
              data={cluster.licenseExpiryDateText}
            />
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

export const DataItem = ({ title = '', data = null }) => (
  <DataItemFlex>
    <Text typography="body2" bold style={{ width: '136px' }}>
      {title}:
    </Text>
    <Text typography="body2">{data}</Text>
  </DataItemFlex>
);

const DataItemFlex = styled(Flex)`
  margin-bottom: ${props => props.theme.space[3]}px;
  @media screen and (max-width: ${props => props.theme.breakpoints.mobile}px) {
    flex-direction: column;
    padding-left: ${props => props.theme.space[2]}px;
  }
`;
