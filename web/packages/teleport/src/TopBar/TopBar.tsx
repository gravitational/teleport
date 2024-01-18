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

import React, { lazy, Suspense, useState } from 'react';
import styled, { useTheme } from 'styled-components';
import { Link } from 'react-router-dom';
import { Flex, Image, Text, TopNav } from 'design';

import { matchPath, useHistory } from 'react-router';

import { BrainIcon } from 'design/SVGIcon';

import { ArrowLeft, Download, Server, SlidersVertical } from 'design/Icon';
import { HoverTooltip } from 'shared/components/ToolTip';

import { AssistViewMode } from 'gen-proto-ts/teleport/userpreferences/v1/assist_pb';

import useTeleport from 'teleport/useTeleport';
import { UserMenuNav } from 'teleport/components/UserMenuNav';
import { useFeatures } from 'teleport/FeaturesContext';
import { NavigationCategory } from 'teleport/Navigation/categories';
import useStickyClusterId from 'teleport/useStickyClusterId';
import cfg from 'teleport/config';

import { useLayout } from 'teleport/Main/LayoutContext';
import { getFirstRouteForCategory } from 'teleport/Navigation/Navigation';
import { useUser } from 'teleport/User/UserContext';

import { Notifications } from './Notifications';
import { ButtonIconContainer } from './Shared';
import logoLight from './logoLight.svg';
import logoDark from './logoDark.svg';

const Assist = lazy(() => import('teleport/Assist'));

export function TopBar({ CustomLogo }: TopBarProps) {
  const ctx = useTeleport();
  const { preferences } = useUser();
  const viewMode = preferences?.assist?.viewMode;
  const { clusterId } = useStickyClusterId();
  const history = useHistory();
  const features = useFeatures();
  const assistEnabled = ctx.getFeatureFlags().assist && ctx.assistEnabled;
  const topBarLinks = features.filter(
    feature =>
      feature.category === NavigationCategory.Resources && feature.topMenuItem
  );

  const [showAssist, setShowAssist] = useState(false);

  const { hasDockedElement } = useLayout();

  // find active feature
  const feature = features
    .filter(feature => Boolean(feature.route))
    .find(f =>
      matchPath(history.location.pathname, {
        path: f.route.path,
        exact: f.route.exact ?? false,
      })
    );

  function handleBack() {
    const firstRouteForCategory = getFirstRouteForCategory(
      features,
      feature.category
    );

    history.push(firstRouteForCategory);
  }

  const resourceTabSelected =
    history?.location?.pathname === cfg.getUnifiedResourcesRoute(clusterId);
  const managementTabSelected =
    feature?.category === NavigationCategory.Management;
  const downloadTabSelected =
    history?.location?.pathname === cfg.routes.downloadCenter;

  return (
    <TopBarContainer
      navigationHidden={feature?.hideNavigation}
      dockedView={showAssist && viewMode === AssistViewMode.DOCKED}
    >
      {!feature?.hideNavigation && (
        <>
          <TeleportLogo CustomLogo={CustomLogo} />
          <Flex
            height="100%"
            css={`
              margin-left: auto;
              @media screen and (min-width: ${p =>
                  p.theme.breakpoints.medium}px) {
                margin-left: 0;
                margin-right: auto;
              }
            `}
          >
            {cfg.isDashboard ? (
              <MainNavItem
                name="Downloads"
                to={cfg.routes.downloadCenter}
                isSelected={downloadTabSelected}
                Icon={Download}
              />
            ) : (
              <MainNavItem
                name="Resources"
                to={cfg.getUnifiedResourcesRoute(clusterId)}
                isSelected={resourceTabSelected}
                Icon={Server}
              />
            )}
            <MainNavItem
              name="Access Management"
              to={getFirstRouteForCategory(
                features,
                NavigationCategory.Management
              )}
              isSelected={managementTabSelected}
              Icon={SlidersVertical}
            />

            {topBarLinks.map(({ topMenuItem, navigationItem }) => {
              const selected = history.location.pathname.includes(
                navigationItem.getLink(clusterId)
              );
              return (
                <NavigationButton
                  key={topMenuItem.title}
                  to={topMenuItem.getLink(clusterId)}
                  selected={selected}
                  title={topMenuItem.title}
                  css={`
                    &:hover {
                      color: red;
                    }
                  `}
                >
                  <topMenuItem.icon
                    color={selected ? 'text.main' : 'text.muted'}
                  />
                </NavigationButton>
              );
            })}
          </Flex>
        </>
      )}
      {feature?.hideNavigation && (
        <ButtonIconContainer onClick={handleBack}>
          <ArrowLeft size="medium" />
        </ButtonIconContainer>
      )}
      <Flex height="100%" alignItems="center">
        {!hasDockedElement && assistEnabled && (
          <ButtonIconContainer onClick={() => setShowAssist(true)}>
            <BrainIcon />
          </ButtonIconContainer>
        )}
        <Notifications />
        <UserMenuNav username={ctx.storeUser.state.username} />
        {showAssist && (
          <Suspense fallback={null}>
            <Assist onClose={() => setShowAssist(false)} />
          </Suspense>
        )}
      </Flex>
    </TopBarContainer>
  );
}

const assistDockedWidth = `width: calc(100% - 520px);`;
export const TopBarContainer = styled(TopNav)`
  position: absolute;
  ${props => (props.dockedView ? assistDockedWidth : 'width: 100%;')}
  display: flex;
  justify-content: space-between;
  background: ${p => p.theme.colors.levels.surface};
  overflow-y: initial;
  overflow-x: none;
  flex-shrink: 0;
  z-index: 10;
  border-bottom: 1px solid ${({ theme }) => theme.colors.spotBackground[0]};

  height: ${p => p.theme.topBarHeight[0]}px;
  @media screen and (min-width: ${p => p.theme.breakpoints.small}px) {
    height: ${p => p.theme.topBarHeight[1]}px;
  }
  @media screen and (min-width: ${p => p.theme.breakpoints.large}px) {
    height: ${p => p.theme.topBarHeight[2]}px;
  }

  box-shadow: 0px 1px 3px 0px rgba(0, 0, 0, 0.12),
    0px 1px 1px 0px rgba(0, 0, 0, 0.14), 0px 2px 1px -1px rgba(0, 0, 0, 0.2);
`;

const TeleportLogo = ({ CustomLogo }: TopBarProps) => {
  const theme = useTheme();

  return (
    <HoverTooltip
      anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }}
      transformOrigin={{ vertical: 'top', horizontal: 'center' }}
      tipContent="Teleport Resources Home"
      css={`
        height: 100%;
      `}
    >
      <Link
        css={`
          cursor: pointer;
          height: 100%;
          display: flex;
          width: 190px;
          @media screen and (min-width: ${p => p.theme.breakpoints.medium}px) {
            width: 256px;
          }

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
            src={theme.type === 'dark' ? logoDark : logoLight}
            alt="teleport logo"
            css={`
              padding-left: ${props => props.theme.space[3]}px;
              height: 18px;
              @media screen and (min-width: ${p =>
                  p.theme.breakpoints.medium}px) {
                height: 28px;
                padding-left: ${props => props.theme.space[4]}px;
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

const NavigationButton = ({
  to,
  selected,
  children,
  title,
  ...props
}: {
  to: string;
  selected: boolean;
  children: React.ReactNode;
  title: string;
}) => {
  const theme = useTheme();
  const selectedBorder = `2px solid ${theme.colors.brand}`;
  const selectedBackground = theme.colors.interactive.tonal.primary[0];

  return (
    <HoverTooltip
      anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }}
      transformOrigin={{ vertical: 'top', horizontal: 'center' }}
      tipContent={title}
      css={`
        height: 100%;
      `}
    >
      <Link
        to={to}
        css={`
          text-decoration: none;
          color: rgba(0, 0, 0, 0.54);
          height: 100%;
          padding-left: 16px;
          padding-right: 16px;
          @media screen and (min-width: ${p => p.theme.breakpoints.large}px) {
            padding-left: 24px;
            padding-right: 24px;
          }
          border-bottom: ${selected ? selectedBorder : 'none'};
          background-color: ${selected ? selectedBackground : 'inherit'};
          &:hover {
            background-color: ${selected
              ? selectedBackground
              : theme.colors.buttons.secondary.default};
          }
        `}
        {...props}
      >
        <Flex
          css={`
            height: 100%;
          `}
          justifyContent="center"
          alignItems="center"
        >
          {children}
        </Flex>
      </Link>
    </HoverTooltip>
  );
};

const MainNavItem = ({
  isSelected,
  to,
  name,
  Icon,
}: {
  isSelected: boolean;
  to: string;
  name: string;
  Icon: (props: { color: string }) => JSX.Element;
}) => {
  return (
    <NavigationButton selected={isSelected} to={to} title={name}>
      <Icon color={isSelected ? 'text.main' : 'text.muted'} />
      <Text
        ml={3}
        fontSize={18}
        fontWeight={500}
        css={`
          display: none;
          @media screen and (min-width: ${p => p.theme.breakpoints.medium}px) {
            display: block;
          }
        `}
        color={isSelected ? 'text.main' : 'text.muted'}
      >
        {name}
      </Text>
    </NavigationButton>
  );
};

export type NavigationItem = {
  title: string;
  path: string;
  Icon: JSX.Element;
};

export type TopBarProps = {
  CustomLogo?: () => React.ReactElement;
  showPoweredByLogo?: boolean;
};
