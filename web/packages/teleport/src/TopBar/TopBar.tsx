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

import React, { type JSX } from 'react';
import { Link, matchPath, useLocation } from 'react-router';
import styled, { css, useTheme } from 'styled-components';

import { Box, breakpointsPx, Flex, Image, Text, TopNav } from 'design';
import * as Icon from 'design/Icon';
import { HoverTooltip } from 'design/Tooltip';
import { useStore } from 'shared/libs/stores';

import { logoSrc } from 'teleport/components/LogoHero/LogoHero';
import { UserMenuNav } from 'teleport/components/UserMenuNav';
import cfg from 'teleport/config';
import { FeatureScopes } from 'teleport/features';
import { useFeatures } from 'teleport/FeaturesContext';
import { useLayout } from 'teleport/Main/LayoutContext';
import { zIndexMap } from 'teleport/Navigation/zIndexMap';
import { Notifications } from 'teleport/Notifications';
import useTeleport from 'teleport/useTeleport';

export function TopBar({
  CustomLogo,
  scopePickerMode,
}: {
  CustomLogo?: () => React.ReactElement;
  scopePickerMode?: boolean;
}) {
  const location = useLocation();
  const features = useFeatures();
  const { currentWidth } = useLayout();
  const ctx = useTeleport();
  const storeUser = useStore(ctx.storeUser);
  const scope = storeUser.getScope();

  // find active feature
  const feature = features.find(
    f =>
      f.route &&
      matchPath(
        { path: f.route.path, end: f.route.exact ?? false },
        location.pathname
      )
  );

  const iconSize =
    currentWidth >= breakpointsPx.medium
      ? navigationIconSizeMedium
      : navigationIconSizeSmall;

  return (
    <TopBarContainer>
      <Flex alignItems="center">
        <TeleportLogo CustomLogo={CustomLogo} withLink={!scopePickerMode} />
        {scope && !feature?.logoOnlyTopbar && (
          <HoverTooltip tipContent="Current scope">
            <Flex alignItems="center">
              <Icon.Contract mr={1} aria-label="scope" />
              <Text typography="body1">{scope}</Text>
            </Flex>
          </HoverTooltip>
        )}
      </Flex>
      {!feature?.logoOnlyTopbar && (
        <Flex height="100%" alignItems="center">
          <Notifications iconSize={iconSize} />
          <UserMenuNav hideFeatures={feature instanceof FeatureScopes} />
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
  @media screen and (min-width: ${p => p.theme.breakpoints.small}) {
    height: ${p => p.theme.topBarHeight[1]}px;
  }
`;

const TeleportLogo = ({
  CustomLogo,
  withLink,
}: {
  CustomLogo?: () => React.ReactElement;
  withLink: boolean;
}) => {
  const theme = useTheme();
  const src = logoSrc(theme.type);
  const logoContent = CustomLogo ? (
    <CustomLogo />
  ) : (
    <Image
      data-testid="teleport-logo"
      src={src}
      alt="Teleport logo"
      css={`
        padding-left: ${props => props.theme.space[3]}px;
        padding-right: ${props => props.theme.space[3]}px;
        height: 18px;
        @media screen and (min-width: ${p => p.theme.breakpoints.small}) {
          height: 28px;
          padding-left: ${props => props.theme.space[4]}px;
          padding-right: ${props => props.theme.space[4]}px;
        }
        @media screen and (min-width: ${p => p.theme.breakpoints.large}) {
          height: 30px;
        }
      `}
    />
  );

  return withLink ? (
    <HoverTooltip placement="bottom" tipContent="Teleport Resources Home">
      <LinkLogoWrapper to={cfg.routes.root}>{logoContent}</LinkLogoWrapper>
    </HoverTooltip>
  ) : (
    <BoxLogoWrapper>{logoContent}</BoxLogoWrapper>
  );
};

const commonLogoWrapperStyles = css`
  display: flex;
  align-items: center;
  height: 100%;
  margin-right: 0px;
`;

const BoxLogoWrapper = styled(Box)`
  ${commonLogoWrapperStyles}
`;

const LinkLogoWrapper = styled(Link)`
  ${commonLogoWrapperStyles}

  cursor: pointer;
  transition: background-color 0.1s linear;
  &:hover {
    background-color: ${p => p.theme.colors.interactive.tonal.primary[0]};
  }
`;

export const navigationIconSizeSmall = 20;
export const navigationIconSizeMedium = 24;

export type NavigationItem = {
  title: string;
  path: string;
  Icon: JSX.Element;
};
