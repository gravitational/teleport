/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React, { useState, useEffect } from 'react';
import styled from 'styled-components';
import { useLocation } from 'react-router';
import { NavLink } from 'react-router-dom';

import { ButtonPrimary, Box, Text, Flex } from 'design';
import { OpenBox, Person } from 'design/Icon';
import TopNavUserMenu from 'design/TopNav/TopNavUserMenu';
import { MenuItemIcon, MenuItem } from 'design/Menu';

import history from 'teleport/services/history';
import cfg from 'teleport/config';
import { NavItem } from 'teleport/stores/storeNav';
import localStorage from 'teleport/services/localStorage';

export function UserMenuNav({ navItems, username, logout }: Props) {
  const { pathname } = useLocation();
  const [open, setOpen] = useState(false);

  const discover = localStorage.getOnboardDiscover();

  // viewingXXX flags to determine what view the user is
  // currently on to determine where the checkmark icon
  // on the dropdown items should be rendered.
  const viewingDiscover = pathname === cfg.routes.discover;
  const viewingUserMenu =
    !viewingDiscover && navItems.some(n => pathname.startsWith(n.getLink()));
  const viewingResources = !viewingUserMenu && !viewingDiscover;

  const firstTimeDiscoverVisit =
    discover && !discover.hasResource && !discover.hasVisited;
  const showDiscoverAlertBubble = !viewingDiscover && firstTimeDiscoverVisit;
  const isFirstTimeDiscoverVisited = viewingDiscover && firstTimeDiscoverVisit;

  useEffect(() => {
    if (isFirstTimeDiscoverVisited) {
      const discover = localStorage.getOnboardDiscover();
      localStorage.setOnboardDiscover({
        ...discover,
        hasVisited: true,
      });
    }
  }, [isFirstTimeDiscoverVisited]);

  const menuItemProps = {
    onClick: closeMenu,
    py: 2,
    as: NavLink,
    exact: true,
  };

  const $userMenuItems = navItems.map((item, index) => {
    const menuPath = item.getLink();
    return (
      <MenuItem {...menuItemProps} key={index} to={menuPath}>
        <FlexedMenuItemIcon as={item.Icon} />
        <FlexSpaceBetween>
          <Text>{item.title}</Text>
          {/* Using 'startsWith' to account for routes that have sub paths eg:
              - main path: /web/account
              - sub paths: /web/account/password, /web/account/twofactor
          */}
          {pathname.startsWith(menuPath) && <Checkmark />}
        </FlexSpaceBetween>
      </MenuItem>
    );
  });

  function showMenu() {
    setOpen(true);
  }

  function closeMenu() {
    setOpen(false);
  }

  function handleLogout() {
    closeMenu();
    logout();
  }

  function handleClickDiscover() {
    if (firstTimeDiscoverVisit) {
      localStorage.setOnboardDiscover({ ...discover, hasVisited: true });
    }

    history.push(cfg.routes.discover);
    closeMenu();
  }

  return (
    <TopNavUserMenu
      menuListCss={menuListCss}
      open={open}
      onShow={showMenu}
      onClose={closeMenu}
      user={username}
    >
      <MenuItem {...menuItemProps} to={cfg.routes.root}>
        <BorderedFlexedMenuItemIcon as={Person} />
        <FlexSpaceBetween>
          <Text>Browse Resources</Text>
          {viewingResources && <Checkmark />}
        </FlexSpaceBetween>
      </MenuItem>
      <MenuItem py={2} onClick={handleClickDiscover}>
        <div
          css={`
            position: relative;
          `}
        >
          <BorderedFlexedMenuItemIcon as={OpenBox} />
          {showDiscoverAlertBubble && (
            <AlertBubble data-testid="alert-bubble" />
          )}
        </div>
        <FlexSpaceBetween>
          <Text>Manage Access</Text>
          {viewingDiscover && <Checkmark />}
        </FlexSpaceBetween>
      </MenuItem>
      <Box
        my={2}
        css={`
          border-bottom: 1px solid #e3e3e3;
        `}
      />
      {$userMenuItems}
      <MenuItem>
        <ButtonPrimary my={3} block onClick={handleLogout}>
          Sign Out
        </ButtonPrimary>
      </MenuItem>
    </TopNavUserMenu>
  );
}

const Checkmark = () => <StyledCheckmark data-testid="checkmark" />;
const StyledCheckmark = styled(Text)(
  props => `
  color: ${props.theme.colors.success};
  font-size: ${props.theme.fontSizes[6]}${'px'};

  :before {
    content: '✓';
  }
`
);

const menuListCss = () => `
  width: 220px;
`;

const FlexedMenuItemIcon = styled(MenuItemIcon)`
  display: flex;
  align-items: center;
  justify-content: center;
`;

const BorderedFlexedMenuItemIcon = styled(FlexedMenuItemIcon)`
  background: #f1eeee;
  border-radius: 4px;
  padding: 3px;
  width: 18px;
  height: 18px;
`;

const AlertBubble = styled.div`
  position: absolute;
  width: 6px;
  height: 6px;
  background: ${({ theme }) => theme.colors.danger};
  border-radius: 100%;
  top: -2px;
  right: 6px;
`;

const FlexSpaceBetween = styled(Flex)`
  width: 100%;
  justify-content: space-between;
`;

type Props = {
  navItems: NavItem[];
  username: string;
  logout(): void;
};
