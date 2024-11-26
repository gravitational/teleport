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

import React from 'react';
import styled, { useTheme } from 'styled-components';
import { Link } from 'react-router-dom';
import { Flex, Image, TopNav } from 'design';
import { matchPath, useHistory } from 'react-router';
import { Theme } from 'design/theme/themes/types';
import { HoverTooltip } from 'shared/components/ToolTip';

import useTeleport from 'teleport/useTeleport';
import { UserMenuNav } from 'teleport/components/UserMenuNav';
import { useFeatures } from 'teleport/FeaturesContext';
import cfg from 'teleport/config';
import { useLayout } from 'teleport/Main/LayoutContext';
import { logos } from 'teleport/components/LogoHero/LogoHero';

import { Notifications } from 'teleport/Notifications';
import { zIndexMap } from 'teleport/Navigation/SideNavigation/zIndexMap';

export function TopBar({ CustomLogo }: TopBarProps) {
  const ctx = useTeleport();
  const history = useHistory();
  const features = useFeatures();
  const { currentWidth } = useLayout();
  const theme: Theme = useTheme();

  // find active feature
  const feature = features.find(
    f =>
      f.route &&
      matchPath(history.location.pathname, {
        path: f.route.path,
        exact: f.route.exact ?? false,
      })
  );

  const iconSize =
    currentWidth >= theme.breakpoints.medium
      ? navigationIconSizeMedium
      : navigationIconSizeSmall;

  return (
    <TopBarContainer navigationHidden={feature?.hideNavigation}>
      <TeleportLogo CustomLogo={CustomLogo} />
      {!feature?.logoOnlyTopbar && (
        <Flex height="100%" alignItems="center">
          <Notifications iconSize={iconSize} />
          <UserMenuNav username={ctx.storeUser.state.username} />
        </Flex>
      )}
    </TopBarContainer>
  );
}

export const TopBarContainer = styled(TopNav)`
  position: fixed;
  width: 100%;
  display: flex;
  justify-content: space-between;
  background: ${p => p.theme.colors.levels.surface};
  overflow-y: initial;
  overflow-x: none;
  flex-shrink: 0;
  z-index: ${zIndexMap.topBar};
  border-bottom: 1px solid ${({ theme }) => theme.colors.spotBackground[1]};

  height: ${p => p.theme.topBarHeight[0]}px;
  @media screen and (min-width: ${p => p.theme.breakpoints.small}px) {
    height: ${p => p.theme.topBarHeight[1]}px;
  }
`;

const TeleportLogo = ({ CustomLogo }: TopBarProps) => {
  const theme = useTheme();
  const src = logos[cfg.edition][theme.type];

  return (
    <HoverTooltip
      anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }}
      transformOrigin={{ vertical: 'top', horizontal: 'center' }}
      tipContent="Teleport Resources Home"
      css={`
        height: 100%;
        margin-right: 0px;
        @media screen and (min-width: ${p => p.theme.breakpoints.medium}px) {
          margin-right: 76px;
        }
        @media screen and (min-width: ${p => p.theme.breakpoints.large}px) {
          margin-right: 67px;
        }
      `}
    >
      <Link
        css={`
          cursor: pointer;
          display: flex;
          transition: background-color 0.1s linear;
          &:hover {
            background-color: ${p =>
              p.theme.colors.interactive.tonal.primary[0]};
          }
          align-items: center;
        `}
        to={cfg.routes.root}
      >
        {CustomLogo ? (
          <CustomLogo />
        ) : (
          <Image
            data-testid="teleport-logo"
            src={src}
            alt="teleport logo"
            css={`
              padding-left: ${props => props.theme.space[3]}px;
              padding-right: ${props => props.theme.space[3]}px;
              height: 18px;
              @media screen and (min-width: ${p =>
                  p.theme.breakpoints.small}px) {
                height: 28px;
                padding-left: ${props => props.theme.space[4]}px;
                padding-right: ${props => props.theme.space[4]}px;
              }
              @media screen and (min-width: ${p =>
                  p.theme.breakpoints.large}px) {
                height: 30px;
              }
            `}
          />
        )}
      </Link>
    </HoverTooltip>
  );
};

export const navigationIconSizeSmall = 20;
export const navigationIconSizeMedium = 24;

export type NavigationItem = {
  title: string;
  path: string;
  Icon: JSX.Element;
};

export type TopBarProps = {
  CustomLogo?: () => React.ReactElement;
  showPoweredByLogo?: boolean;
};
