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
import { NavLink, Link } from 'react-router-dom';
import { Flex, Image } from 'design';

import cfg from 'teleport/config';

import SideNavItemIcon from './SideNavItemIcon';
import SideNavItem from './SideNavItem';
import SideNavItemGroup from './SideNavItemGroup';
import logoSvg from './logo';
import useSideNav from './useSideNav';
import SideNavExternalLink from './SideNavExternalLink';

export default function Container() {
  const state = useSideNav();
  return <SideNav {...state} />;
}

export function SideNav(props: ReturnType<typeof useSideNav>) {
  const { items, path } = props;
  const $items = items.map((item, index) => {
    const isChild = item.items.length > 0;
    if (isChild) {
      return <SideNavItemGroup path={path} item={item} key={index} />;
    }

    return item.isExternalLink ? (
      <SideNavExternalLink key={index} icon={item.Icon} href={item.route}>
        {item.title}
      </SideNavExternalLink>
    ) : (
      <SideNavItem key={index} as={NavLink} exact={item.exact} to={item.route}>
        <SideNavItemIcon as={item.Icon} />
        {item.title}
      </SideNavItem>
    );
  });

  return (
    <Nav>
      <Logo />
      <Content>{$items}</Content>
    </Nav>
  );
}

export const Logo = () => (
  <LogoItem pl="4" width="208px" as={Link} to={cfg.routes.root}>
    <Image src={logoSvg} mx="3" maxHeight="24px" maxWidth="160px" />
  </LogoItem>
);

const LogoItem = styled(Flex)(
  props => `
  min-height: 56px;
  align-items: center;
  cursor: pointer;
  outline: none;
  text-decoration: none;
  width: 100%;
  &:hover {
    background ${props.theme.colors.levels.elevated};
    color ${props.theme.colors.text.contrast};
  }
`
);

export const Nav = styled.nav`
  background: ${props => props.theme.colors.levels.surface};
  border-right: 1px solid ${props => props.theme.colors.levels.sunkenSecondary};
  overflow: auto;
  height: 100%;
  display: flex;
  flex-direction: column;
  min-width: var(--sidebar-width);
  width: var(--sidebar-width);
  box-sizing: border-box;
`;

export const Content = styled.div`
  display: flex;
  flex-direction: column;
  overflow: auto;
`;
