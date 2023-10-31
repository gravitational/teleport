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

import React, { useEffect } from 'react';
import styled from 'styled-components';
import { Box, ButtonPrimary, Flex, Text } from 'design';

import * as types from 'teleterm/ui/services/workspacesService';
import Document from 'teleterm/ui/Document';
import { useAppContext } from 'teleterm/ui/appContextProvider';

import * as uri from 'teleterm/ui/uri';
import { routing } from 'teleterm/ui/uri';

import { Cluster } from 'teleterm/services/tshd/types';

import { UnifiedResources } from './UnifiedResources';

export default function DocumentCluster(props: {
  visible: boolean;
  doc: types.DocumentCluster;
}) {
  const { clusterUri } = props.doc;
  const appCtx = useAppContext();

  const rootCluster =
    appCtx.clustersService.findRootClusterByResource(clusterUri);
  const cluster = appCtx.clustersService.findCluster(clusterUri);

  const clusterName = cluster?.name || routing.parseClusterName(clusterUri);
  useEffect(() => {
    // because we don't wait for the leaf clusters to fetch before we show them,
    // we can't access `actualName` when the cluster document is created
    appCtx.workspacesService
      .getWorkspaceDocumentService(routing.ensureRootClusterUri(clusterUri))
      .update(props.doc.uri, {
        title: clusterName,
      });
  }, [appCtx.workspacesService, clusterName, clusterUri, props.doc.uri]);

  function logIn(): void {
    appCtx.modalsService.openRegularDialog({
      kind: 'cluster-connect',
      clusterUri,
      reason: undefined,
      prefill: undefined,
      onCancel: () => {},
      onSuccess: () => {},
    });
  }

  return (
    <Document visible={props.visible}>
      <ClusterState
        clusterUri={clusterUri}
        rootCluster={rootCluster}
        cluster={cluster}
        onLogin={logIn}
      />
    </Document>
  );
}

function ClusterState(props: {
  clusterUri: uri.ClusterUri;
  rootCluster: Cluster;
  cluster: Cluster | undefined;
  onLogin(): void;
}) {
  if (!props.rootCluster.connected) {
    return (
      <RequiresLogin clusterUri={props.clusterUri} onLogin={props.onLogin} />
    );
  }

  if (!props.cluster) {
    return <NotFound clusterUri={props.clusterUri} />;
  }

  if (props.cluster.leaf && !props.cluster.connected) {
    return <LeafDisconnected clusterUri={props.clusterUri} />;
  }

  return (
    <Layout>
      <UnifiedResources clusterUri={props.clusterUri} />
    </Layout>
  );
}

function RequiresLogin(props: { clusterUri: uri.ClusterUri; onLogin(): void }) {
  return (
    <Flex
      flexDirection="column"
      mx="auto"
      justifyContent="center"
      alignItems="center"
    >
      <Text typography="h4" color="text.main" bold>
        {props.clusterUri}
        <Text as="span" typography="h5">
          {` cluster is offline`}
        </Text>
      </Text>
      <ButtonPrimary mt={4} width="100px" onClick={props.onLogin}>
        Connect
      </ButtonPrimary>
    </Flex>
  );
}

// TODO(ravicious): Add a button for syncing the leaf clusters list.
// https://github.com/gravitational/teleport.e/issues/863
function LeafDisconnected(props: { clusterUri: uri.ClusterUri }) {
  return (
    <Flex flexDirection="column" mx="auto" alignItems="center">
      <Text typography="h5">{props.clusterUri}</Text>
      <Text as="span" typography="h5">
        trusted cluster is offline
      </Text>
    </Flex>
  );
}

function NotFound(props: { clusterUri: uri.ClusterUri }) {
  return (
    <Flex flexDirection="column" mx="auto" alignItems="center">
      <Text typography="h5">{props.clusterUri}</Text>
      <Text as="span" typography="h5">
        Not Found
      </Text>
    </Flex>
  );
}

const Layout = styled(Box).attrs({ mx: 'auto', px: 5, pt: 4 })`
  flex-direction: column;
  display: flex;
  flex: 1;
`;
