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
import { Box, Flex } from 'design';

export default function ItemList({ items, curIndex, onSelect, ...rest }) {
  const $items = items.map((i, index) => {
    const { name } = i;
    return (
      <Item
        py="3"
        pl="4"
        name={name}
        key={index}
        active={index === curIndex}
        onClick={() => onSelect(index)}
      />
    )});

  return (
    <Flex
      flex="1"
      flexDirection="column"
      color="text.primary"
      bold
      children={$items}
      style={{overflow: "auto"}}
      {...rest} />
  )
}

const Item = ({ name, dirty, active, onClick, ...rest}) => (
  <StyledItem typography="body1" bold
    as="button"
    dirty={dirty}
    active={active}
    onClick={onClick}
    {...rest}
  >
    <span>{name}</span>
  </StyledItem>
)

function fromProps({ theme, active }){
  let styles = {
    borderBottom: `1px solid ${theme.colors.primary.dark }`,
    '&:hover, &:focus': {
      color: theme.colors.primary.contrastText
    },
    '&:hover': {
      backgroundColor: theme.colors.primary.main,
    }
  }

  if(active){
    styles = {
      ...styles,
      backgroundColor: theme.colors.primary.main,
      color: theme.colors.primary.contrastText
    }
  }

  return styles;
}


const StyledItem = styled(Box)`
  border: none;
  position: relative;
  display: inline-flex;
  outline: none;
  text-align: inherit;
  text-decoration: none;
  outline: none;
  text-decoration: none;
  color: inherit;
  cursor: pointer;
  background-color: transparent;

  > span {
    overflow: hidden;
    text-overflow: ellipsis;
  }

  ${fromProps}
  ${typography}
`