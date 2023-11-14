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
import { Box, ButtonPrimary, Flex, Text, Alert } from 'design';
import { useAsync, Attempt } from 'shared/hooks/useAsync';

import * as types from 'teleterm/ui/services/workspacesService';
import Document from 'teleterm/ui/Document';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import { retryWithRelogin } from 'teleterm/ui/utils';
import { Cluster } from 'teleterm/services/tshd/types';

import * as uri from 'teleterm/ui/uri';
import { routing } from 'teleterm/ui/uri';

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

  const [clusterSyncAttempt, syncCluster] = useAsync(() =>
    retryWithRelogin(appCtx, clusterUri, () =>
      appCtx.clustersService.syncRootCluster(
        routing.ensureRootClusterUri(clusterUri)
      )
    )
  );

  return (
    <Document visible={props.visible}>
      <ClusterState
        clusterName={clusterName}
        clusterUri={clusterUri}
        rootCluster={rootCluster}
        cluster={cluster}
        syncCluster={syncCluster}
        clusterSyncAttempt={clusterSyncAttempt}
      />
    </Document>
  );
}

function ClusterState(props: {
  clusterUri: uri.ClusterUri;
  clusterName: string;
  rootCluster: Cluster;
  cluster: Cluster | undefined;
  syncCluster(): void;
  clusterSyncAttempt: Attempt<void>;
}) {
  if (!props.rootCluster.connected) {
    return (
      <RequiresLogin
        clusterName={props.clusterName}
        syncCluster={props.syncCluster}
        clusterSyncAttempt={props.clusterSyncAttempt}
      />
    );
  }

  if (!props.cluster) {
    return <NotFound clusterName={props.clusterName} />;
  }

  if (props.cluster.leaf && !props.cluster.connected) {
    return (
      <LeafDisconnected
        clusterName={props.clusterName}
        syncCluster={props.syncCluster}
        clusterSyncAttempt={props.clusterSyncAttempt}
      />
    );
  }

  return (
    <Layout>
      <UnifiedResources clusterUri={props.clusterUri} />
    </Layout>
  );
}

function RequiresLogin(props: {
  clusterName: string;
  syncCluster(): void;
  clusterSyncAttempt: Attempt<void>;
}) {
  return (
    <PrintState
      clusterName={props.clusterName}
      clusterState="Cluster is offline."
      action={{
        attempt: props.clusterSyncAttempt,
        label: 'Connect',
        run: props.syncCluster,
      }}
    />
  );
}

function LeafDisconnected(props: {
  clusterName: string;
  syncCluster(): void;
  clusterSyncAttempt: Attempt<void>;
}) {
  return (
    <PrintState
      clusterName={props.clusterName}
      clusterState="Trusted cluster is offline."
      action={{
        attempt: props.clusterSyncAttempt,
        label: 'Refresh cluster status',
        run: props.syncCluster,
      }}
    />
  );
}

function NotFound(props: { clusterName: string }) {
  return (
    <PrintState
      clusterName={props.clusterName}
      clusterState="Cluster not found."
    />
  );
}

function PrintState(props: {
  clusterName: string;
  clusterState: string;
  action?: {
    label: string;
    run(): void;
    attempt: Attempt<void>;
  };
}) {
  return (
    <Flex
      flexDirection="column"
      m="auto"
      justifyContent="center"
      alignItems="center"
    >
      {props.action && props.action.attempt.status === 'error' && (
        <Alert>{props.action.attempt.statusText}</Alert>
      )}
      <Text typography="h4" bold>
        {props.clusterName}
      </Text>
      <Text>{props.clusterState}</Text>
      {props.action && (
        <ButtonPrimary
          mt={4}
          onClick={props.action.run}
          disabled={props.action.attempt.status === 'processing'}
        >
          {props.action.label}
        </ButtonPrimary>
      )}
    </Flex>
  );
}

const Layout = styled(Box).attrs({ mx: 'auto', px: 5, pt: 4 })`
  flex-direction: column;
  display: flex;
  flex: 1;
`;
