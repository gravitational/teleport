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

import { useEffect } from 'react';
import styled from 'styled-components';

import { Alert, Box, ButtonPrimary, Flex, H2, Text } from 'design';
import { Attempt, makeEmptyAttempt, useAsync } from 'shared/hooks/useAsync';

import { Cluster } from 'teleterm/services/tshd/types';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import Document from 'teleterm/ui/Document';
import * as types from 'teleterm/ui/services/workspacesService';
import { DocumentClusterQueryParams } from 'teleterm/ui/services/workspacesService';
import * as uri from 'teleterm/ui/uri';
import { routing } from 'teleterm/ui/uri';
import { retryWithRelogin } from 'teleterm/ui/utils';

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

  const refreshRootCluster = () =>
    appCtx.commandLauncher.executeCommand('cluster-connect', {
      clusterUri: routing.ensureRootClusterUri(clusterUri),
    });

  return (
    <Document visible={props.visible}>
      <ClusterState
        clusterName={clusterName}
        clusterUri={clusterUri}
        rootCluster={rootCluster}
        cluster={cluster}
        syncCluster={syncCluster}
        refreshRootCluster={refreshRootCluster}
        clusterSyncAttempt={clusterSyncAttempt}
        queryParams={props.doc.queryParams}
        docUri={props.doc.uri}
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
  refreshRootCluster(): void;
  clusterSyncAttempt: Attempt<void>;
  queryParams: DocumentClusterQueryParams;
  docUri: uri.DocumentUri;
}) {
  if (!props.rootCluster.connected) {
    return (
      <PrintState
        clusterName={props.clusterName}
        clusterState="Cluster is offline."
        action={{
          attempt: makeEmptyAttempt(),
          label: 'Connect',
          // props.syncCluster should not be used here, as it shows a relogin modal only when the
          // GetCluster RPC returns an error resolvable with relogin.
          // The cached cluster client might continue to be able to reach the cluster even after
          // props.rootCluster.connected returns false. This is the case when
          // disconnect_expired_cert is set to false (the default).
          //
          // So the two might disagree because `props.rootCluster.connected` is set from the
          // ListRootClusters RPC which reads data from disk and clearly sees that the cert has
          // expired, whereas the GetCluster RPC gets the cluster info by connecting to the cluster
          // itself.
          //
          // Instead, always explicitly show the cluster login modal on click.
          run: props.refreshRootCluster,
        }}
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
        // Syncing the root cluster is going to refresh the list of leaf clusters, possibly updating
        // props.cluster.connected.
        syncCluster={props.syncCluster}
        clusterSyncAttempt={props.clusterSyncAttempt}
      />
    );
  }

  return (
    <Layout>
      <UnifiedResources
        clusterUri={props.clusterUri}
        docUri={props.docUri}
        queryParams={props.queryParams}
      />
    </Layout>
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
      mx="auto"
      mb="auto"
      alignItems="center"
      px={4}
      css={`
        top: 11%;
        position: relative;
      `}
    >
      {props.action && props.action.attempt.status === 'error' && (
        <Alert>{props.action.attempt.statusText}</Alert>
      )}
      <H2 mb={1}>{props.clusterName}</H2>
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

const Layout = styled(Box).attrs({ mx: 'auto', px: 4, pt: 3 })`
  flex-direction: column;
  display: flex;
  flex: 1;
`;
