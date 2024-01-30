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

import React, { useState } from 'react';
import styled, { useTheme } from 'styled-components';

import { Moon, Sun, ChevronDown, Logout as LogoutIcon } from 'design/Icon';
import { Text } from 'design';
import { useRefClickOutside } from 'shared/hooks/useRefClickOutside';

import { Theme } from 'gen-proto-ts/teleport/userpreferences/v1/theme_pb';

import session from 'teleport/services/websession';
import { useFeatures } from 'teleport/FeaturesContext';
import { useTeleport } from 'teleport';
import { useUser } from 'teleport/User/UserContext';
import {
  Dropdown,
  DropdownItem,
  DropdownItemButton,
  DropdownItemLink,
  DropdownItemIcon,
  DropdownDivider,
  STARTING_TRANSITION_DELAY,
  INCREMENT_TRANSITION_DELAY,
} from 'teleport/components/Dropdown';

interface UserMenuNavProps {
  username: string;
}

const Container = styled.div`
  position: relative;
  align-self: center;
  padding-left: ${props => props.theme.space[3]}px;
  padding-right: ${props => props.theme.space[3]}px;
  &:hover {
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
`;

const Username = styled(Text)`
  color: ${props => props.theme.colors.text.main};
  font-size: 14px;
  font-weight: 400;
  padding-right: 40px;
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

const Arrow = styled.div`
  line-height: 0;

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

  const ref = useRefClickOutside<HTMLDivElement>({ open, setOpen });

  const ctx = useTeleport();
  const clusterId = ctx.storeUser.getClusterId();
  const features = useFeatures();

  const onThemeChange = () => {
    const nextTheme =
      preferences.theme === Theme.LIGHT ? Theme.DARK : Theme.LIGHT;

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
      <DropdownItem open={open} key={index} $transitionDelay={transitionDelay}>
        <DropdownItemLink
          to={item.topMenuItem.getLink(clusterId)}
          onClick={() => setOpen(false)}
        >
          <DropdownItemIcon>{<item.topMenuItem.icon />}</DropdownItemIcon>
          {item.topMenuItem.title}
        </DropdownItemLink>
      </DropdownItem>
    );

    transitionDelay += INCREMENT_TRANSITION_DELAY;
  }

  return (
    <Container ref={ref}>
      <UserInfo onClick={() => setOpen(!open)} open={open}>
        <StyledAvatar>{initial}</StyledAvatar>

        <Username>{username}</Username>

        <Arrow open={open}>
          <ChevronDown size="medium" />
        </Arrow>
      </UserInfo>

      <Dropdown open={open}>
        {items}

        <DropdownDivider />

        {/* Hide ability to switch themes if the theme is a custom theme */}
        {!theme.isCustomTheme && (
          <DropdownItem open={open} $transitionDelay={transitionDelay}>
            <DropdownItemButton onClick={onThemeChange}>
              <DropdownItemIcon>
                {preferences.theme === Theme.DARK ? <Sun /> : <Moon />}
              </DropdownItemIcon>
              Switch to {preferences.theme === Theme.DARK ? 'Light' : 'Dark'}{' '}
              Theme
            </DropdownItemButton>
          </DropdownItem>
        )}

        <DropdownItem open={open} $transitionDelay={transitionDelay}>
          <DropdownItemButton onClick={() => session.logout()}>
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
