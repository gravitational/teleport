/*
Copyright 2019-2022 Gravitational, Inc.

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
import { Indicator, Flex, Box } from 'design';

import NodeList from 'teleport/components/NodeList';
import QuickLaunch from 'teleport/components/QuickLaunch';
import ErrorMessage from 'teleport/components/AgentErrorMessage';
import Document from 'teleport/Console/Document';

import * as stores from 'teleport/Console/stores/types';

import ClusterSelector from './ClusterSelector';
import useNodes from './useNodes';

type Props = {
  visible: boolean;
  doc: stores.DocumentNodes;
};

export default function DocumentNodes(props: Props) {
  const { doc, visible } = props;
  const {
    fetchedData,
    fetchNext,
    fetchPrev,
    pageSize,
    params,
    setParams,
    setSort,
    pathname,
    replaceHistory,
    fetchStatus,
    attempt,
    createSshSession,
    changeCluster,
    getNodeSshLogins,
    onLabelClick,
    pageIndicators,
  } = useNodes(doc);

  function onLoginMenuSelect(
    e: React.MouseEvent,
    login: string,
    serverId: string
  ) {
    // allow to open a new browser tab (not the console one) when requested
    const newBrowserTabRequested = e.ctrlKey || e.metaKey;
    if (!newBrowserTabRequested) {
      e.preventDefault();
      createSshSession(login, serverId);
    }
  }

  function onQuickLaunchEnter(login: string, serverId: string) {
    createSshSession(login, serverId);
  }

  function onLoginMenuOpen(serverId: string) {
    return getNodeSshLogins(serverId);
  }

  function onChangeCluster(newClusterId: string) {
    changeCluster(newClusterId);
  }

  return (
    <Document visible={visible}>
      <Container mx="auto" mt="4" px="5">
        <Flex justifyContent="space-between" mb="4" alignItems="end">
          <ClusterSelector
            value={doc.clusterId}
            width="336px"
            maxMenuHeight={200}
            mr="20px"
            onChange={onChangeCluster}
          />
          <QuickLaunch width="240px" onPress={onQuickLaunchEnter} />
        </Flex>
        {attempt.status === 'processing' && (
          <Box textAlign="center" m={10}>
            <Indicator />
          </Box>
        )}
        {attempt.status === 'failed' && (
          <ErrorMessage message={attempt.statusText} />
        )}
        {attempt.status !== 'processing' && (
          <NodeList
            nodes={fetchedData.agents}
            onLoginMenuOpen={onLoginMenuOpen}
            onLoginSelect={onLoginMenuSelect}
            fetchNext={fetchNext}
            fetchPrev={fetchPrev}
            fetchStatus={fetchStatus}
            pageIndicators={pageIndicators}
            pageSize={pageSize}
            params={params}
            setParams={setParams}
            setSort={setSort}
            pathname={pathname}
            replaceHistory={replaceHistory}
            onLabelClick={onLabelClick}
          />
        )}
      </Container>
    </Document>
  );
}

const Container = styled(Box)`
  flex-direction: column;
  display: flex;
  flex: 1;
  max-width: 1024px;
  height: fit-content;
  ::after {
    content: ' ';
    padding-bottom: 24px;
  }
`;
