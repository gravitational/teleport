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

import React from 'react';
import styled from 'styled-components';
import { typography } from 'design/system';
import { Box } from 'design';

export default function Tabs({ items, activeTab, dirtyTabs, onSelect, ...styledProps }) {
  const $items = items.map((i, index) => {
    const { name } = i;
    return (
      <TabItem
        name={name}
        key={index}
        dirty={dirtyTabs[index]}
        active={index === activeTab}
        onClick={() => onSelect(index)}
      />
    )});

  return (
    <StyledTab bg="primary.dark" typography="h5" color="text.secondary" bold children={$items} {...styledProps} />
  )
}

const TabItem = ({ name, dirty, active, onClick}) => (
  <StyledTabItem typography="h4" mr={1} mb={2}
    as="button"
    title={name}
    dirty={dirty}
    active={active}
    onClick={onClick}
  >
    <span>{name}</span>
    {dirty && <StyledDirtyFlag>{" *"}</StyledDirtyFlag>}
  </StyledTabItem>
)

function fromProps({ theme, active }){
  let styles = {
    border: 'none',
    borderRight: `1px solid ${theme.colors.bgTerminal }`,
    '&:hover, &:focus': {
      color: theme.colors.primary.contrastText,
      transition: 'color .3s'
    }
  }

  if(active){
    styles = {
      ...styles,
      backgroundColor: theme.colors.bgTerminal,
      color: theme.colors.primary.contrastText,
      fontWeight: 'bold',
      transition: 'none'
    }
  }

  return styles;
}

const StyledDirtyFlag = styled.span`
  position: absolute;
`

const StyledTabItem = styled(Box)`
  position: relative;
  display: inline-block;
  outline: none;
  text-align: center;
  text-decoration: none;
  outline: none;
  margin: 0;
  text-decoration: none;
  color: inherit;
  cursor: pointer;
  line-height: 32px;
  padding: 0 12px;
  background-color: transparent;
  ${fromProps}
  max-width: 240px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
`

const StyledTab = styled(Box)`
  border-radius: 4px;

  display: flex;
  flex-wrap: wrap;
  align-items: center;
  flex-shrink: 0;
  overflow: hidden;

  > * {
    flex: 1;
    flex-basis: 0;
    flex-grow: 1;
  }
  ${typography}
`