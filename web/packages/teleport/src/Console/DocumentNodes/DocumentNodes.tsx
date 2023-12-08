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
import { Indicator, Flex, Box } from 'design';

import NodeList from 'teleport/components/NodeList';
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
