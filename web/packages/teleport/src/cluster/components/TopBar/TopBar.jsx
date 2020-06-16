/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';
import styled from 'styled-components';
import { ArrowRight } from 'design/Icon';
import { NavLink } from 'react-router-dom';
import { Text, ButtonOutlined, Flex } from 'design';
import TopBar from 'teleport/components/TopBar';
import cfg from 'teleport/config';

export default function ClusterTopBar({ onClickClusterInfo }) {
  const clusterId = cfg.clusterName;
  return (
    <TopBar>
      <Flex alignItems="center">
        <BreadCrumbLink
          ml="6"
          typography="h5"
          color="text.secondary"
          as={NavLink}
          to={cfg.routes.app}
          title="Go back to cluster list"
        >
          All Clusters
        </BreadCrumbLink>
        <ArrowRight mx="2" fontSize={12} color="text.secondary" />
        <Text
          mr="3"
          typography="h5"
          color="text.primary"
          style={{ maxWidth: '200px', whiteSpace: 'nowrap' }}
          title={clusterId}
        >
          {clusterId}
        </Text>
        <ButtonOutlined
          size="small"
          style={{ whiteSpace: 'nowrap' }}
          onClick={onClickClusterInfo}
        >
          Cluster Info
        </ButtonOutlined>
      </Flex>
    </TopBar>
  );
}

const BreadCrumbLink = styled(Text)`
  text-decoration: none;
  font-size: 14px;
  font-weight: 300;
  white-space: nowrap;

  &:hover {
    color: ${props => props.theme.colors.text.primary};
  }
`;
