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

  removeCluster(clusterUri: string): void;

  selectCluster(clusterUri: string): void;

  addCluster(): void;

  logout(): void;
}

export function IdentityList(props: IdentityListProps) {
  return (
    <Container py="12px">
      <Flex px={'24px'} pb={2} justifyContent="space-between">
        <Box>
          <Text bold>{props.loggedInUser.name}</Text>
          <Text typography="body2" color="text.secondary">
            {props.loggedInUser?.rolesList?.join(', ')}
          </Text>
        </Box>
        <ButtonIcon onClick={props.addCluster} title="Add cluster">
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
              onSelect={() => props.selectCluster(i.uri)}
              onRemove={() => props.removeCluster(i.uri)}
            />
          ))}
        </Box>
        <Separator />
        <Box px={'12px'}>
          <LogoutItem index={props.clusters.length + 1} logout={props.logout} />
        </Box>
      </KeyboardArrowsNavigation>
    </Container>
  );
}

const Container = styled(Box)`
  background: ${props => props.theme.colors.primary.dark};
  width: 280px;
`;

const Separator = styled.div`
  background: ${props => props.theme.colors.primary.lighter};
  margin: 0 16px;
  height: 1px;
`;
