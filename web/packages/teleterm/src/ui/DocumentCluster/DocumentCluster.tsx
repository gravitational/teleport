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
  appCtx.clustersService.useState();

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
      <Layout>
        <ClusterState
          clusterName={clusterName}
          clusterUri={clusterUri}
          rootCluster={rootCluster}
          cluster={cluster}
          onLogin={logIn}
        />
      </Layout>
    </Document>
  );
}

function ClusterState(props: {
  clusterUri: uri.ClusterUri;
  clusterName: string;
  rootCluster: Cluster;
  cluster: Cluster | undefined;
  onLogin(): void;
}) {
  if (!props.rootCluster.connected) {
    return (
      <RequiresLogin clusterName={props.clusterName} onLogin={props.onLogin} />
    );
  }

  if (!props.cluster) {
    return <NotFound clusterName={props.clusterName} />;
  }

  if (props.cluster.leaf && !props.cluster.connected) {
    return <LeafDisconnected clusterName={props.clusterName} />;
  }

  return <UnifiedResources clusterUri={props.clusterUri} />;
}

function RequiresLogin(props: { clusterName: string; onLogin(): void }) {
  return (
    <PrintState
      clusterName={props.clusterName}
      clusterState="Cluster is offline."
      children={
        <ButtonPrimary mt={4} onClick={props.onLogin}>
          Connect
        </ButtonPrimary>
      }
    />
  );
}

// TODO(ravicious): Add a button for syncing the leaf clusters list.
// https://github.com/gravitational/teleport.e/issues/863
function LeafDisconnected(props: { clusterName: string }) {
  return (
    <PrintState
      clusterName={props.clusterName}
      clusterState="Trusted cluster is offline."
    />
  );
}

function NotFound(props: { clusterName: string }) {
  return (
    <PrintState
      clusterName={props.clusterName}
      clusterState="Cluster is not found."
    />
  );
}

function PrintState(props: {
  clusterName: string;
  clusterState: string;
  children?: React.ReactElement;
}) {
  return (
    <Flex
      flexDirection="column"
      m="auto"
      justifyContent="center"
      alignItems="center"
    >
      <Text typography="h4" bold>
        {props.clusterName}
      </Text>
      <Text as="span" typography="h5">
        {props.clusterState}
      </Text>
      {props.children}
    </Flex>
  );
}

const Layout = styled(Box).attrs({ mx: 'auto', px: 5, pt: 4 })`
  flex-direction: column;
  display: flex;
  flex: 1;
`;
