/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { useRef, useState } from 'react';
import styled, { useTheme } from 'styled-components';

import { Box, Text } from 'design';
import { ChevronDown, Logout as LogoutIcon, Moon, Sun } from 'design/Icon';
import { Theme } from 'gen-proto-ts/teleport/userpreferences/v1/theme_pb';
import { useRefClickOutside } from 'shared/hooks/useRefClickOutside';

import { useTeleport } from 'teleport';
import {
  Dropdown,
  DropdownDivider,
  DropdownItem,
  DropdownItemButton,
  DropdownItemIcon,
  DropdownItemLink,
  INCREMENT_TRANSITION_DELAY,
  STARTING_TRANSITION_DELAY,
} from 'teleport/components/Dropdown';
import { useFeatures } from 'teleport/FeaturesContext';
import { focusOutsideTarget } from 'teleport/lib/util/eventTarget';
import session from 'teleport/services/websession';
import { getCurrentTheme, getNextTheme } from 'teleport/ThemeProvider';
import { DeviceTrustStatus } from 'teleport/TopBar/DeviceTrustStatus';
import { useUser } from 'teleport/User/UserContext';

interface UserMenuNavProps {
  username: string;
}

const USER_MENU_DROPDOWN_ID = 'tb-user-menu';

const Container = styled.div`
  position: relative;
  align-self: center;
  padding-left: ${props => props.theme.space[3]}px;
  padding-right: ${props => props.theme.space[3]}px;
  &:hover,
  &:focus-within {
    background: ${props => props.theme.colors.spotBackground[0]};
  }
  height: 100%;
`;

const UserInfo = styled.div`
  height: 100%;
  display: flex;
  align-items: center;
  border-radius: 5px;
  cursor: pointer;
  user-select: none;
  position: relative;
  outline: none;
`;

const Username = styled(Text)`
  color: ${props => props.theme.colors.text.main};
  font-size: 14px;
  font-weight: 400;
  display: none;
  @media screen and (min-width: ${p => p.theme.breakpoints.large}px) {
    display: inline-flex;
  }
`;

const StyledAvatar = styled.div`
  align-items: center;
  background: ${props => props.theme.colors.brand};
  color: ${props => props.theme.colors.text.primaryInverse};
  border-radius: 50%;
  @media screen and (min-width: ${p => p.theme.breakpoints.medium}px) {
    margin-right: 16px;
    height: 32px;
    max-width: 32px;
    min-width: 32px;
  }
  display: flex;
  font-size: 14px;
  font-weight: bold;
  justify-content: center;
  width: 100%;
  height: 24px;
  max-width: 24px;
  min-width: 24px;
`;

const Arrow = styled.div<{ open?: boolean }>`
  line-height: 0;
  padding-left: ${p => p.theme.space[3]}px;

  svg {
    transform: ${p => (p.open ? 'rotate(-180deg)' : 'none')};
    transition: 0.1s linear transform;
  }

  display: none;
  @media screen and (min-width: ${p => p.theme.breakpoints.medium}px) {
    display: inline-flex;
  }
`;

export function UserMenuNav({ username }: UserMenuNavProps) {
  const [open, setOpen] = useState(false);
  const theme = useTheme();

  const { preferences, updatePreferences } = useUser();

  const outsideClickRef = useRefClickOutside<HTMLDivElement>({ open, setOpen });
  const dropdownRef = useRef<HTMLDivElement>(null);

  const ctx = useTeleport();
  const clusterId = ctx.storeUser.getClusterId();
  const features = useFeatures();
  const currentTheme = getCurrentTheme(preferences.theme);
  const nextTheme = getNextTheme(preferences.theme);

  const onThemeChange = () => {
    updatePreferences({ theme: nextTheme });
    setOpen(false);
  };

  const initial =
    username && username.length ? username.trim().charAt(0).toUpperCase() : '';

  const topMenuItems = features.filter(
    feature => Boolean(feature.topMenuItem) && feature.category === undefined
  );

  const items = [];

  let transitionDelay = STARTING_TRANSITION_DELAY;
  for (const [index, item] of topMenuItems.entries()) {
    items.push(
      <DropdownItem
        open={open}
        key={index}
        $transitionDelay={transitionDelay}
        role="menuitem"
      >
        <DropdownItemLink
          to={item.topMenuItem.getLink(clusterId)}
          onClick={() => setOpen(false)}
          onKeyUp={e => (e.key === 'Enter' || e.key === ' ') && setOpen(false)}
        >
          <DropdownItemIcon>{<item.topMenuItem.icon />}</DropdownItemIcon>
          {item.topMenuItem.title}
        </DropdownItemLink>
      </DropdownItem>
    );

    transitionDelay += INCREMENT_TRANSITION_DELAY;
  }

  return (
    <Container ref={outsideClickRef}>
      <UserInfo
        onClick={() => setOpen(!open)}
        onKeyUp={e => {
          if (e.key === 'Enter' || e.key === ' ') {
            setOpen(!open);
            return;
          }
          if (e.key === 'Tab' && open) {
            // move to first focusable item in dropdown
            dropdownRef.current
              ?.querySelector<HTMLElement>('a, div[role="button"]')
              ?.focus();
          }
        }}
        onBlur={e =>
          focusOutsideTarget(e, dropdownRef.current) && setOpen(false)
        }
        tabIndex={0}
        role="button"
        aria-label="User Menu"
        aria-haspopup="menu"
        aria-expanded={open}
        aria-controls={USER_MENU_DROPDOWN_ID}
      >
        <StyledAvatar>{initial}</StyledAvatar>

        <Username>{username}</Username>
        <Box ml={3}>
          <DeviceTrustStatus iconOnly />
        </Box>

        <Arrow open={open}>
          <ChevronDown size="medium" />
        </Arrow>
      </UserInfo>

      <Dropdown
        open={open}
        ref={dropdownRef}
        role="menu"
        id={USER_MENU_DROPDOWN_ID}
        onBlur={e =>
          !e.currentTarget.contains(e.relatedTarget as Node) && setOpen(false)
        }
      >
        <DeviceTrustStatus />
        {items}

        <DropdownDivider />

        {/* Hide ability to switch themes if the theme is a custom theme */}
        {!theme.isCustomTheme && (
          <DropdownItem
            open={open}
            $transitionDelay={transitionDelay}
            role="menuitem"
          >
            <DropdownItemButton
              onClick={onThemeChange}
              onKeyUp={e =>
                (e.key === 'Enter' || e.key === ' ') && onThemeChange()
              }
              tabIndex={0}
            >
              <DropdownItemIcon>
                {currentTheme === Theme.DARK ? <Sun /> : <Moon />}
              </DropdownItemIcon>
              Switch to {currentTheme === Theme.DARK ? 'Light' : 'Dark'} Theme
            </DropdownItemButton>
          </DropdownItem>
        )}

        <DropdownItem
          open={open}
          $transitionDelay={transitionDelay}
          role="menuitem"
        >
          <DropdownItemButton
            onClick={() => session.logout()}
            onKeyUp={e =>
              (e.key === 'Enter' || e.key === ' ') && session.logout()
            }
            tabIndex={0}
          >
            <DropdownItemIcon>
              <LogoutIcon />
            </DropdownItemIcon>
            Logout
          </DropdownItemButton>
        </DropdownItem>
      </Dropdown>
    </Container>
  );
}
