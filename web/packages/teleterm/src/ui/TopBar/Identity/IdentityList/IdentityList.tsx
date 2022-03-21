import React from 'react';
import { KeyboardArrowsNavigation } from 'teleterm/ui/components/KeyboardArrowsNavigation';
import styled from 'styled-components';
import { Add } from 'design/Icon';
import { Box, ButtonIcon, Flex, Text } from 'design';
import { LoggedInUser } from 'teleterm/services/tshd/types';
import { ClusterItem } from './ClusterItem';
import { LogoutItem } from './LogoutItem';
import { IdentityRootCluster } from '../useIdentity';

interface IdentityListProps {
  loggedInUser: LoggedInUser;
  clusters: IdentityRootCluster[];

  onRemoveCluster(clusterUri: string): void;

  onSelectCluster(clusterUri: string): void;

  onAddCluster(): void;

  onLogout(): void;
}

export function IdentityList(props: IdentityListProps) {
  return (
    <>
      <Flex px={'24px'} pb={2} justifyContent="space-between">
        <Box>
          <Text bold>{props.loggedInUser?.name}</Text>
          <Text typography="body2" color="text.secondary">
            {props.loggedInUser?.rolesList?.join(', ')}
          </Text>
        </Box>
        <ButtonIcon onClick={props.onAddCluster} title="Add cluster">
          <Add />
        </ButtonIcon>
      </Flex>
      <Separator />
      <KeyboardArrowsNavigation>
        <Box px={'12px'}>
          {props.clusters.map((i, index) => (
            <ClusterItem
              key={i.uri}
              index={index}
              isSelected={i.active}
              title={i.name}
              isSyncing={i.clusterSyncStatus}
              onSelect={() => props.onSelectCluster(i.uri)}
              onRemove={() => props.onRemoveCluster(i.uri)}
            />
          ))}
        </Box>
        {props.loggedInUser && (
          <>
            <Separator />
            <Box px={'12px'}>
              <LogoutItem
                index={props.clusters.length + 1}
                onLogout={props.onLogout}
              />
            </Box>
          </>
        )}
      </KeyboardArrowsNavigation>
    </>
  );
}

const Separator = styled.div`
  background: ${props => props.theme.colors.primary.lighter};
  margin: 0 16px;
  height: 1px;
`;
