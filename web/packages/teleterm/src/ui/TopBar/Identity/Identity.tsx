import React, { useRef, useState } from 'react';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import { Box, Flex, Text } from 'design';
import Popover from 'design/Popover';
import { ClusterItem } from './ClusterItem';
import styled from 'styled-components';
import { ListItem } from 'teleterm/ui/components/ListItem';
import { Cluster } from 'teleterm/services/tshd/types';
import { SortDesc } from 'design/Icon';
import AppContext from 'teleterm/ui/appContext';

export function Identity() {
  const shortInfoRef = useRef<HTMLButtonElement>();
  const [isLongInfoOpened, setIsLongInfoOpened] = useState(false);
  const ctx = useAppContext();

  ctx.clustersService.useState();
  ctx.workspacesService.useState();

  function tryToChangeContext(clusterUri: string): void {
    ctx.workspacesService.setActiveWorkspace(clusterUri);
  }

  const activeRootCluster = getActiveRootCluster(ctx);

  const $clustersItems = ctx.clustersService
    .getClusters()
    .filter(c => !c.leaf)
    .map(cluster => ({
      active: cluster.uri === ctx.workspacesService.getRootClusterUri(),
      title: cluster.name,
      uri: cluster.uri,
      connected: cluster.connected,
      clusterSyncStatus: ctx.clustersService.getClusterSyncStatus(cluster.uri),
    }))
    .map(i => (
      <ClusterItem
        key={i.uri}
        isActive={i.active}
        title={i.title}
        syncing={i.clusterSyncStatus.syncing}
        onClick={() => tryToChangeContext(i.uri)}
      />
    ));

  const loggedInUser = activeRootCluster?.loggedInUser;
  return (
    <>
      <ShortInfoButton
        ref={shortInfoRef}
        onClick={() => setIsLongInfoOpened(prevState => !prevState)}
      >
        <ShortInfoContainer alignItems="center">
          <Flex flexDirection="column">
            {loggedInUser ? (
              <>
                <Text typography="paragraph2" bold>
                  {loggedInUser.name}
                </Text>
                <Text
                  typography="paragraph2"
                  fontWeight="regular"
                  mb="0"
                  css={{ opacity: 0.7, lineHeight: '14px' }}
                >
                  {activeRootCluster.name}
                </Text>
              </>
            ) : (
              'Select a cluster'
            )}
          </Flex>
          <SortDesc ml={24} />
        </ShortInfoContainer>
      </ShortInfoButton>
      <Popover
        open={isLongInfoOpened}
        anchorEl={shortInfoRef.current}
        anchorOrigin={{ vertical: 'bottom' }}
        onClose={() => setIsLongInfoOpened(false)}
      >
        <Background>
          <Padding>
            Roles:{' '}
            {loggedInUser?.rolesList?.map((role, index) => (
              <Text as="span" bold key={index}>
                {role}
              </Text>
            ))}
          </Padding>
          <Separator />
          {$clustersItems}
          <Separator />
          <ListItem>Logout</ListItem>
        </Background>
      </Popover>
    </>
  );
}

function getActiveRootCluster(ctx: AppContext): Cluster | undefined {
  const clusterUri = ctx.workspacesService.getRootClusterUri();
  if (!clusterUri) {
    return;
  }
  return ctx.clustersService.findCluster(clusterUri);
}

const ShortInfoButton = styled.button`
  background: inherit;
  border: none;
`;

const ShortInfoContainer = styled(Flex)`
  cursor: pointer;
  color: ${props => props.theme.colors.text.primary};
`;

const Padding = styled.div`
  padding: 0 16px;
`;

const Background = styled(Box)`
  background: ${props => props.theme.colors.primary.dark};
  padding: 16px 0;
`;

const Separator = styled.div`
  background: ${props => props.theme.colors.primary.lighter};
  margin: 0 16px;
  height: 1px;
`;
