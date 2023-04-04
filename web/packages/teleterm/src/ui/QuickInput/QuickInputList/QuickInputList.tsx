/*
Copyright 2018 Gravitational, Inc.

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

import React, { useEffect, useRef } from 'react';
import styled from 'styled-components';
import { Box, Flex, Label, Text } from 'design';
import { makeLabelTag } from 'teleport/components/formatters';

import { Cli, Server, Person, Database } from 'design/Icon';

import * as types from 'teleterm/ui/services/quickInput/types';

const QuickInputList = React.forwardRef<HTMLElement, Props>((props, ref) => {
  // Ideally, this property would be described by the suggestion object itself rather than depending
  // on `kind`. But for now we need it just for a single suggestion kind anyway.
  const shouldSuggestionsStayInPlace =
    props.items[0]?.kind === 'suggestion.cmd';
  const activeItemRef = useRef<HTMLDivElement>();
  const { items, activeItem } = props;
  if (items.length === 0) {
    return null;
  }

  useEffect(() => {
    // `false` - bottom of the element will be aligned to the bottom of the visible area of the scrollable ancestor
    activeItemRef.current?.scrollIntoView(false);
  }, [activeItem]);

  const $items = items.map((r, index) => {
    const Cmpt = ComponentMap[r.kind] || UnknownItem;
    const isActive = index === activeItem;
    return (
      <StyledItem
        data-attr={index}
        ref={isActive ? activeItemRef : null}
        $active={isActive}
        key={` ${index}`}
      >
        <Cmpt item={r} />
      </StyledItem>
    );
  });

  function handleClick(e: React.SyntheticEvent) {
    const el = e.target;
    if (el instanceof Element) {
      const itemEl = el.closest('[data-attr]');
      props.onPick(parseInt(itemEl.getAttribute('data-attr')));
    }
  }

  return (
    <StyledGlobalSearchResults
      position={shouldSuggestionsStayInPlace ? null : props.position}
      ref={ref}
      tabIndex={-1}
      data-attr="quickpicker.list"
      onClick={handleClick}
    >
      {$items}
    </StyledGlobalSearchResults>
  );
});

export default QuickInputList;

function CmdItem(props: { item: types.SuggestionCmd }) {
  return (
    <Flex alignItems="baseline">
      <SquareIconBackground color="#512FC9">
        <Cli fontSize="10px" />
      </SquareIconBackground>
      {/* Equivalent of flex-shrink: 0, but styled-system doesn't support flex-shrink. */}
      <Box flex="0 0 auto" mr={2}>
        {props.item.data.displayName}
      </Box>
      <Box color="text.secondary">{props.item.data.description}</Box>
    </Flex>
  );
}

function SshLoginItem(props: { item: types.SuggestionSshLogin }) {
  return (
    <Flex alignItems="baseline">
      <SquareIconBackground color="#FFAB00">
        <Person fontSize="10px" />
      </SquareIconBackground>
      <Box mr={2}>{props.item.data}</Box>
    </Flex>
  );
}

function ServerItem(props: { item: types.SuggestionServer }) {
  const { hostname, labelsList } = props.item.data;
  const $labels = labelsList.map((label, index) => (
    <Label mr="1" key={index} kind="secondary">
      {makeLabelTag(label)}
    </Label>
  ));

  return (
    <Flex alignItems="baseline" p={1} minWidth="300px">
      <SquareIconBackground color="#4DB2F0">
        <Server fontSize="10px" />
      </SquareIconBackground>
      <Flex flexDirection="column" ml={1}>
        <Box mr={2}>{hostname}</Box>
        <Box>{$labels}</Box>
      </Flex>
    </Flex>
  );
}

function DatabaseItem(props: { item: types.SuggestionDatabase }) {
  const db = props.item.data;
  const $labels = db.labelsList.map((label, index) => (
    <Label mr="1" key={index} kind="secondary">
      {makeLabelTag(label)}
    </Label>
  ));

  return (
    <Flex alignItems="baseline" p={1} minWidth="300px">
      <SquareIconBackground color="#4DB2F0">
        <Database fontSize="10px" />
      </SquareIconBackground>
      <Flex flexDirection="column" ml={1} flex={1}>
        <Flex justifyContent="space-between" alignItems="center">
          <Box mr={2}>{db.name}</Box>
          <Box mr={2}>
            <Text typography="body2" fontSize={0}>
              {db.type}/{db.protocol}
            </Text>
          </Box>
        </Flex>
        <Box>{$labels}</Box>
      </Flex>
    </Flex>
  );
}

function UnknownItem(props: { item: types.Suggestion }) {
  const { kind } = props.item;
  return <div>unknown kind: {kind} </div>;
}

const StyledItem = styled.div(({ theme, $active }) => {
  return {
    '&:hover, &:focus': {
      cursor: 'pointer',
      background: theme.colors.levels.elevated,
    },

    padding: '2px 8px',
    color: theme.colors.text.contrast,
    background: $active
      ? theme.colors.levels.surfaceSecondary
      : theme.colors.levels.sunken,
  };
});

const StyledGlobalSearchResults = styled.div(({ theme, position }) => {
  return {
    boxShadow: '8px 8px 18px rgb(0 0 0)',
    color: theme.colors.text.contrast,
    background: theme.colors.levels.surface,
    boxSizing: 'border-box',
    marginTop: '42px',
    left: position ? position + 'px' : 0,
    display: 'block',
    transition: '0.12s',
    position: 'absolute',
    borderRadius: '4px',
    fontSize: '12px',
    listStyle: 'none outside none',
    textShadow: 'none',
    zIndex: '1000',
    maxHeight: '350px',
    overflow: 'auto',
  };
});

const ComponentMap: Record<
  types.Suggestion['kind'],
  React.FC<{ item: types.Suggestion }>
> = {
  ['suggestion.cmd']: CmdItem,
  ['suggestion.ssh-login']: SshLoginItem,
  ['suggestion.server']: ServerItem,
  ['suggestion.database']: DatabaseItem,
};

type Props = {
  items: types.Suggestion[];
  activeItem: number;
  position: number;
  onPick(index: number): void;
};

const SquareIconBackground = styled(Box)`
  background: ${props => props.color};
  display: flex;
  align-items: center;
  justify-content: center;
  height: 14px;
  width: 14px;
  margin-right: 8px;
  border-radius: 2px;
  padding: 4px;
`;
