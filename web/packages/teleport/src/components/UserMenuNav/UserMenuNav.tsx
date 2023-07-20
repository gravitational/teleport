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

import React, { useCallback, useEffect, useRef, useState } from 'react';
import styled, { css } from 'styled-components';

import { Moon, Sun } from 'design/Icon';
import { ChevronDownIcon } from 'design/SVGIcon/ChevronDown';
import { Text } from 'design';
import { LogoutIcon } from 'design/SVGIcon';
import { NavLink } from 'react-router-dom';

import session from 'teleport/services/websession';
import { useFeatures } from 'teleport/FeaturesContext';
import { useTeleport } from 'teleport';
import { useUser } from 'teleport/User/UserContext';
import { ThemePreference } from 'teleport/services/userPreferences/types';

interface UserMenuNavProps {
  username: string;
}

const Container = styled.div`
  position: relative;
  align-self: center;
  margin-right: 30px;
`;

const UserInfo = styled.div`
  display: flex;
  align-items: center;
  padding: 8px;
  border-radius: 5px;
  cursor: pointer;
  user-select: none;
  position: relative;

  &:hover {
    background: ${props => props.theme.colors.spotBackground[0]};
  }
`;

const Username = styled(Text)`
  color: ${props => props.theme.colors.text.main}
  font-size: 14px;
  font-weight: 400;
  padding-right: 40px;
`;

const StyledAvatar = styled.div`
  align-items: center;
  background: ${props => props.theme.colors.brand};
  color: ${props => props.theme.colors.text.primaryInverse};
  border-radius: 50%;
  display: flex;
  font-size: 14px;
  font-weight: bold;
  justify-content: center;
  height: 32px;
  margin-right: 16px;
  width: 100%;
  max-width: 32px;
  min-width: 32px;
`;

const Arrow = styled.div`
  position: absolute;
  right: 10px;
  top: 50%;
  transform: translate(0, -50%);
  line-height: 0;

  svg {
    transform: ${p => (p.open ? 'rotate(-180deg)' : 'none')};
    transition: 0.1s linear transform;
  }
`;

interface OpenProps {
  open: boolean;
}

const Dropdown = styled.div<OpenProps>`
  position: absolute;
  display: flex;
  flex-direction: column;
  padding: 10px 15px;
  background: ${({ theme }) => theme.colors.levels.elevated};
  box-shadow: ${({ theme }) => theme.boxShadow[1]};
  border-radius: 5px;
  width: 265px;
  right: 0;
  top: 43px;
  z-index: 999;
  opacity: ${p => (p.open ? 1 : 0)};
  visibility: ${p => (p.open ? 'visible' : 'hidden')};
  transform-origin: top right;
  transition:
    opacity 0.2s ease,
    visibility 0.2s ease,
    transform 0.3s cubic-bezier(0.45, 0.6, 0.5, 1.25);
  transform: ${p =>
    p.open ? 'scale(1) translate(0, 12px)' : 'scale(.8) translate(0, 4px)'};
`;

const DropdownItem = styled.div`
  line-height: 1;
  font-size: 14px;
  color: ${props => props.theme.colors.text.main};
  cursor: pointer;
  border-radius: 4px;
  margin-bottom: 5px;
  opacity: ${p => (p.open ? 1 : 0)};
  transition:
    transform 0.3s ease,
    opacity 0.7s ease;
  transform: translate3d(${p => (p.open ? 0 : '20px')}, 0, 0);

  &:hover {
    background: ${props => props.theme.colors.spotBackground[0]};
  }

  &:last-of-type {
    margin-bottom: 0;
  }
`;

const commonDropdownItemStyles = css`
  opacity: 0.8;
  align-items: center;
  display: flex;
  padding: 10px 10px;
  color: ${props => props.theme.colors.text.main};
  text-decoration: none;
  transition: opacity 0.15s ease-in;

  &:hover {
    opacity: 1;
  }
`;

const DropdownItemButton = styled.div`
  ${commonDropdownItemStyles};
`;

const DropdownItemLink = styled(NavLink)`
  ${commonDropdownItemStyles};
`;

const DropdownItemIcon = styled.div`
  margin-right: 16px;
  line-height: 0;
`;

const DropdownDivider = styled.div`
  height: 1px;
  background: ${props => props.theme.colors.spotBackground[1]};
  margin: 0 5px 5px 5px;
`;

export function UserMenuNav({ username }: UserMenuNavProps) {
  const [open, setOpen] = useState(false);

  const { preferences, updatePreferences } = useUser();

  const ref = useRef<HTMLDivElement>();

  const ctx = useTeleport();
  const clusterId = ctx.storeUser.getClusterId();
  const features = useFeatures();

  const onThemeChange = () => {
    const nextTheme =
      preferences.theme === ThemePreference.Light
        ? ThemePreference.Dark
        : ThemePreference.Light;

    updatePreferences({ theme: nextTheme });
    setOpen(false);
  };

  const initial =
    username && username.length ? username.trim().charAt(0).toUpperCase() : '';

  const handleClickOutside = useCallback(
    (event: MouseEvent) => {
      if (ref.current && !ref.current.contains(event.target as HTMLElement)) {
        setOpen(false);
      }
    },
    [ref.current]
  );

  useEffect(() => {
    if (open) {
      document.addEventListener('mousedown', handleClickOutside);

      return () => {
        document.removeEventListener('mousedown', handleClickOutside);
      };
    }
  }, [ref, open, handleClickOutside]);

  const topMenuItems = features.filter(feature => Boolean(feature.topMenuItem));

  const items = [];

  let transitionDelay = 80;
  for (const [index, item] of topMenuItems.entries()) {
    items.push(
      <DropdownItem
        open={open}
        key={index}
        style={{
          transitionDelay: `${transitionDelay}ms`,
        }}
      >
        <DropdownItemLink
          to={item.topMenuItem.getLink(clusterId)}
          onClick={() => setOpen(false)}
        >
          <DropdownItemIcon>{item.topMenuItem.icon}</DropdownItemIcon>
          {item.topMenuItem.title}
        </DropdownItemLink>
      </DropdownItem>
    );

    transitionDelay += 20;
  }

  return (
    <Container ref={ref}>
      <UserInfo onClick={() => setOpen(!open)} open={open}>
        <StyledAvatar>{initial}</StyledAvatar>

        <Username>{username}</Username>

        <Arrow open={open}>
          <ChevronDownIcon />
        </Arrow>
      </UserInfo>

      <Dropdown open={open}>
        {items}

        <DropdownDivider />

        <DropdownItem
          open={open}
          style={{
            transitionDelay: `${transitionDelay}ms`,
          }}
        >
          <DropdownItemButton onClick={onThemeChange}>
            <DropdownItemIcon>
              {preferences.theme === ThemePreference.Light ? <Sun /> : <Moon />}
            </DropdownItemIcon>
            Switch to{' '}
            {preferences.theme === ThemePreference.Dark ? 'Light' : 'Dark'}{' '}
            Theme
          </DropdownItemButton>
        </DropdownItem>
        <DropdownItem
          open={open}
          style={{
            transitionDelay: `${transitionDelay}ms`,
          }}
        >
          <DropdownItemButton onClick={() => session.logout()}>
            <DropdownItemIcon>
              <LogoutIcon size={16} />
            </DropdownItemIcon>
            Logout
          </DropdownItemButton>
        </DropdownItem>
      </Dropdown>
    </Container>
  );
}
