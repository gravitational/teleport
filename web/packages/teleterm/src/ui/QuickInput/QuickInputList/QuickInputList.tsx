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

import React from 'react';
import styled from 'styled-components';
import { Label, Box, Text, Flex } from 'design';
import { makeLabelTag } from 'teleport/components/formatters';
import * as types from 'teleterm/ui/services/quickInput/types';

const QuickInputList = React.forwardRef<HTMLElement, Props>((props, ref) => {
  const { items, activeItem } = props;
  if (items.length === 0) {
    return null;
  }

  const $items = items.map((r, index) => {
    const Cmpt = ComponentMap[r.kind] || UnknownItem;
    return (
      <StyledItem
        data-attr={index}
        $active={index === activeItem}
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
    <Flex alignItems="center">
      <Box mr={2}>{props.item.data.displayName}</Box>
      <Box color="text.placeholder">{props.item.data.description}</Box>
    </Flex>
  );
}

function SshLoginItem(props: { item: types.SuggestionSshLogin }) {
  return (
    <Flex alignItems="center">
      <Box mr={2}>{props.item.data}</Box>
    </Flex>
  );
}

function ServerItem(props: { item: types.SuggestionServer }) {
  const { hostname, uri, labelsList } = props.item.data;
  const $labels = labelsList.map((label, index) => (
    <Label mr="1" key={index} kind="secondary">
      {makeLabelTag(label)}
    </Label>
  ));

  return (
    <div>
      <Flex alignItems="center">
        <Box mr={2}>{hostname}</Box>
        <Box color="text.placeholder">{uri}</Box>
      </Flex>
      {$labels}
    </div>
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
      background: theme.colors.primary.lighter,
    },

    borderBottom: `2px solid ${theme.colors.primary.main}`,
    padding: '2px 8px',
    color: theme.colors.primary.contrastText,
    background: $active
      ? theme.colors.primary.lighter
      : theme.colors.primary.light,
  };
});

const StyledGlobalSearchResults = styled.div(({ theme }) => {
  return {
    boxShadow: '8px 8px 18px rgb(0 0 0)',
    color: theme.colors.primary.contrastText,
    background: theme.colors.primary.light,
    boxSizing: 'border-box',
    width: '600px',
    marginTop: '32px',
    display: 'block',
    position: 'absolute',
    border: '1px solid ' + theme.colors.action.hover,
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
};

type Props = {
  items: types.Suggestion[];
  activeItem: number;
  onPick(index: number): void;
};
