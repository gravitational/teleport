import React from 'react';
import { ListItem } from 'teleterm/ui/components/ListItem';
import { Flex, Text } from 'design';
import { ListAddCheck } from 'design/Icon';
import styled from 'styled-components';
import LinearProgress from 'teleterm/ui/components/LinearProgress';

interface ClusterItemProps {
  isActive: boolean;
  title: string;
  onClick(): void;
  syncing: boolean;
}

export function ClusterItem(props: ClusterItemProps) {
  return (
    <ListItem onClick={props.onClick}>
      <Flex justifyContent="space-between" alignItems="center" width="100%">
        <Text typography="body1" title={props.title}>
          {props.title}
        </Text>
        {props.isActive ? <ActiveCheck /> : null}
        {props.syncing && <LinearProgress />}
      </Flex>
    </ListItem>
  );
}

const ActiveCheck = styled(ListAddCheck)`
  color: ${props => props.theme.colors.progressBarColor};
`;
