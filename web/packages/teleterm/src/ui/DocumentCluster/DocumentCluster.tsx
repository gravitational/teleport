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

import React, { useEffect, useState } from 'react';
import styled from 'styled-components';
import { Box, ButtonPrimary, Flex, Text } from 'design';

import * as types from 'teleterm/ui/services/workspacesService';
import Document from 'teleterm/ui/Document';
import { useAppContext } from 'teleterm/ui/appContextProvider';

import { routing } from 'teleterm/ui/uri';

import ClusterCtx, {
  ClusterContextProvider,
  useClusterContext,
} from './clusterContext';
import ClusterResources from './ClusterResources';

export default function Container(props: DocumentProps) {
  const { clusterUri } = props.doc;
  const appCtx = useAppContext();
  const [clusterCtx] = useState(
    () => new ClusterCtx(appCtx, clusterUri, props.doc.uri)
  );

  useEffect(() => {
    // because we don't wait for the leaf clusters to fetch before we show them
    // we can't access `actualName` when cluster document is created
    appCtx.workspacesService
      .getWorkspaceDocumentService(routing.ensureRootClusterUri(clusterUri))
      .update(props.doc.uri, {
        title: clusterCtx.state.clusterName,
      });
  }, [clusterCtx.state.clusterName]);

  useEffect(() => {
    return () => clusterCtx.dispose();
  }, []);

  return (
    <ClusterContextProvider value={clusterCtx}>
      <Document visible={props.visible}>
        <Cluster />
      </Document>
    </ClusterContextProvider>
  );
}

export function Cluster() {
  const clusterCtx = useClusterContext();
  const state = clusterCtx.useState();

  if (state.status === 'requires_login') {
    return (
      <RequiresLogin
        clusterUri={clusterCtx.clusterUri}
        onLogin={clusterCtx.login}
      />
    );
  }

  if (state.status === 'not_found') {
    return <NotFound clusterUri={clusterCtx.clusterUri} />;
  }

  if (state.leaf && !state.leafConnected) {
    return <LeafDisconnected clusterUri={clusterCtx.clusterUri} />;
  }

  return (
    <Layout mx="auto" px={5} pt={2} height="100%">
      <ClusterResources />
    </Layout>
  );
}

function RequiresLogin(props: { clusterUri: string; onLogin(): void }) {
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
function LeafDisconnected(props: { clusterUri: string }) {
  return (
    <Flex flexDirection="column" mx="auto" alignItems="center">
      <Text typography="h5">{props.clusterUri}</Text>
      <Text as="span" typography="h5">
        trusted cluster is offline
      </Text>
    </Flex>
  );
}

function NotFound(props: { clusterUri: string }) {
  return (
    <Flex flexDirection="column" mx="auto" alignItems="center">
      <Text typography="h5">{props.clusterUri}</Text>
      <Text as="span" typography="h5">
        Not Found
      </Text>
    </Flex>
  );
}

type DocumentProps = {
  visible: boolean;
  doc: types.DocumentCluster;
};

const Layout = styled(Box)`
  flex-direction: column;
  display: flex;
  flex: 1;
  max-width: 1248px;

  ::after {
    content: ' ';
    padding-bottom: 24px;
  }
`;
