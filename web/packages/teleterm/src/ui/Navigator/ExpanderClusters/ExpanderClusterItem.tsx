import React from 'react';
import Expander, { ExpanderHeader, ExpanderContent } from './../Expander';
import { ClusterNavItem } from './types';
import NavItem from 'teleterm/ui/Navigator/NavItem';
import LinearProgress from 'teleterm/ui/components/LinearProgress';
import { Flex, Text, Box } from 'design';

const simpleItemPadding = 5;
const expandedItemPadding = 7;

export function ExpanderClusterItem(props: ExpanderClusterItem) {
  const { item } = props;

  if (item.leaves?.length) {
    return <ExpanderClusterWithLeaves {...props} />;
  }
  return <ClusterItem {...props} pl={simpleItemPadding} />;
}

function ExpanderClusterWithLeaves(props: ExpanderClusterItem) {
  const { item } = props;

  return (
    <Expander>
      <ExpanderHeader pl="18px">
        <ClusterItem {...props} />
      </ExpanderHeader>
      <ExpanderContent>
        <Box>
          {item.leaves.map(tc => (
            <ClusterItem
              {...props}
              item={tc}
              key={tc.clusterUri}
              pl={expandedItemPadding}
              onOpen={props.onOpen}
            />
          ))}
        </Box>
      </ExpanderContent>
    </Expander>
  );
}

function ClusterItem(props: ExpanderClusterItem & { pl?: number }) {
  const { title, syncing, clusterUri, connected } = props.item;
  const titleColor = connected ? 'text.primary' : 'text.placeholder';

  function handleClick() {
    props.onOpen(clusterUri);
  }

  return (
    <NavItem
      active={props.item.active}
      item={props.item}
      pl={props.pl}
      onClick={handleClick}
    >
      <Flex
        alignItems="center"
        justifyContent="space-between"
        flex="1"
        width="100%"
        onContextMenu={props.onContextMenu}
        style={{ position: 'relative' }}
      >
        <Flex
          justifyContent="center"
          alignItems="center"
          color={titleColor}
          minWidth={0}
        >
          <Text typography="body1" title={title}>
            {title}
            {syncing && <LinearProgress />}
          </Text>
        </Flex>
      </Flex>
    </NavItem>
  );
}

type ExpanderClusterItem = {
  item: ClusterNavItem;
  onContextMenu(): void;
  onOpen(clusterUri: string): void;
};
