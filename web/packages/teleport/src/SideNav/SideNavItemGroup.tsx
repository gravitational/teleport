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
import { NavLink } from 'react-router-dom';
import { matchPath } from 'react-router';

import * as Icons from 'design/Icon';

import SideNavItemIcon from './SideNavItemIcon';
import SideNavItem from './SideNavItem';
import { Item } from './useSideNav';

const SideNavItemGroup: React.FC<{ path: string; item: Item }> = props => {
  const { item, path } = props;
  const hasSelectedChild = isChildActive(path, item);
  // Auto expand on initial render if a child is selected.
  const [expanded, setExpanded] = React.useState(() =>
    isChildActive(path, item)
  );

  React.useEffect(() => {
    // Ensures parent is expanded, if a child is selected.
    if (hasSelectedChild && !expanded) {
      setExpanded(true);
    }
  }, [hasSelectedChild]);

  const ArrowIcon = expanded ? Icons.ArrowDown : Icons.ArrowRight;
  const style = {
    display: expanded ? 'block' : 'none',
  };

  const $children = item.items.map((i, index) => {
    return (
      <SideNavItem
        key={index}
        $nested={true}
        as={NavLink}
        exact={i.exact}
        to={i.route}
      >
        <StyledMarker className="marker"></StyledMarker>
        <SideNavItemIcon as={i.Icon} fontSize="2" mr={2} />
        {i.title}
      </SideNavItem>
    );
  });

  const className = hasSelectedChild ? 'actives' : '';

  return (
    <>
      <StyledGroup
        className={className}
        as="button"
        onClick={() => setExpanded(!expanded)}
      >
        <SideNavItemIcon as={item.Icon} />
        {item.title}
        <ArrowIcon
          ml="auto"
          mr={-2}
          color="inherit"
          style={{ fontSize: '14px' }}
        />
      </StyledGroup>
      <StyledChildrenContainer style={style}>
        {$children}
      </StyledChildrenContainer>
    </>
  );
};

export default SideNavItemGroup;

function isChildActive(url: string, item: Item) {
  return item.items.some(
    i =>
      !!matchPath(url, {
        path: i.route,
        exact: i.exact,
      })
  );
}

const fromTheme = ({ theme }) => {
  return {
    fontSize: '12px',
    fontWeight: theme.regular,
    fontFamily: theme.font,
    paddingLeft: theme.space[9] + 'px',
    paddingRight: theme.space[5] + 'px',
    background: theme.colors.levels.surface,
    color: theme.colors.text.secondary,

    '&.active': {
      borderLeftColor: theme.colors.brand.accent,
      background: theme.colors.levels.elevated,
      color: theme.colors.text.contrast,
      '.marker': {
        background: theme.colors.brand.accent,
      },
    },

    '&:hover': {
      background: theme.colors.levels.elevated,
    },

    '&:hover, &:focus': {
      color: theme.colors.text.contrast,
    },

    minHeight: '56px',
  };
};

const StyledChildrenContainer = styled.div`
  background: ${props =>
    `linefar-gradient(140deg, ${props.theme.colors.levels.elevated}, ${props.theme.colors.levels.surface});`};
`;

const StyledMarker = styled.div`
  height: 8px;
  width: 8px;
  position: absolute;
  top: 16px;
  left: 26px;
`;

const StyledGroup = styled.div`
  margin: 0;
  box-sizing: border-box;
  display: flex;
  align-items: center;
  justify-content: flex-start;
  border: none;
  border-left: 4px solid transparent;
  cursor: pointer;
  outline: none;
  text-decoration: none;
  width: 100%;
  line-height: 24px;
  ${fromTheme}
`;
