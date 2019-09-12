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
import { NavLink } from 'gravity/components/Router';
import { typography, color } from 'design/system';
import { Box } from 'design';

const Tabs = ({ children }) => {
  return (
    <StyledTab
      typography="h5"
      color="text.primary"
      bold
      children={children} />
  )
}

export const TabItem = ({ to, title, exact }) => (
  <StyledTabItem typography="h4" mr={6} mb={2}
    as={NavLink}
    to={to}
    exact={exact}
  >
    {title}
  </StyledTabItem>
)

const StyledTabItem = styled(Box)`
  outline: none;
  text-decoration: none;
  color: inherit;
  cursor: pointer;

  &:hover, &.active, &:focus {
    color: ${ props => props.theme.colors.primary.contrastText};
  }

  &.active {
    border-bottom: 4px solid ${({theme}) => theme.colors.accent};
    padding-top: 4px;
  }
`

const StyledTab = styled.div`
  display: flex;
  align-items: center;
  flex-shrink: 0;
  ${typography}
  ${color}
`

export default Tabs;