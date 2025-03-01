/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import React, { PropsWithChildren, ReactNode } from 'react';
import { NavLink } from 'react-router-dom';
import styled, { css, useTheme } from 'styled-components';

import { Box, ButtonIcon, Flex, P2, Text } from 'design';
import { ArrowLineLeft } from 'design/Icon';
import { Theme } from 'design/theme';
import { HoverTooltip, IconTooltip } from 'design/Tooltip';

import { SlidingSidePanel } from 'teleport/components/SlidingSidePanel';
import cfg from 'teleport/config';

import { CategoryIcon } from './CategoryIcon';
import { NavigationSection, NavigationSubsection } from './Navigation';
import { zIndexMap } from './zIndexMap';

type SharedSectionProps = {
  section: NavigationSection;
  $active: boolean;
  isExpanded: boolean;
  onExpandSection: () => void;
};

/**
 * DefaultSection is a NavigationSection with default children automatically generated from the subsections it contains,
 * this will just be a regular list of subsection items (eg. Identity section).
 */
export function DefaultSection({
  $active,
  section,
  isExpanded,
  onNavigationItemClick,
  previousExpandedSection,
  stickyMode,
  toggleStickyMode,
  currentPageSection,
  currentView,
  onExpandSection,
}: SharedSectionProps & {
  currentView?: NavigationSubsection;
  onNavigationItemClick?: () => void;
  currentPageSection?: NavigationSection;
  stickyMode: boolean;
  toggleStickyMode: () => void;
  previousExpandedSection: NavigationSection;
}) {
  return (
    <>
      <CategoryButton
        $active={$active}
        onMouseEnter={onExpandSection}
        onFocus={onExpandSection}
        isExpanded={isExpanded}
        tabIndex={section.standalone ? 0 : -1}
      >
        <CategoryIcon category={section.category} />
        {section.category}
      </CategoryButton>

      <RightPanel
        isVisible={isExpanded}
        skipAnimation={!!previousExpandedSection}
        id={`panel-${section.category}`}
        onFocus={onExpandSection}
      >
        <Flex
          flexDirection="column"
          justifyContent="space-between"
          height="100%"
        >
          <Box
            css={`
              overflow-y: auto;
              padding: 3px;
            `}
          >
            <RightPanelHeader
              title={section.category}
              stickyMode={stickyMode}
              toggleStickyMode={toggleStickyMode}
              canToggleStickyMode={!!currentPageSection}
            />
            {!section.standalone &&
              section.subsections.map(subsection => (
                <SubsectionItem
                  $active={currentView?.route === subsection.route}
                  to={subsection.route}
                  exact={subsection.exact}
                  key={subsection.title}
                  onClick={() => {
                    onNavigationItemClick();
                  }}
                >
                  <subsection.icon size={16} />
                  <P2>{subsection.title}</P2>
                </SubsectionItem>
              ))}
          </Box>
          {cfg.edition === 'oss' && <AGPLFooter />}
          {cfg.edition === 'community' && <CommunityFooter />}
        </Flex>
      </RightPanel>
    </>
  );
}

/**
 * CustomChildrenSection is a NavigationSection with custom children (e.g. search section).
 */
export function CustomChildrenSection({
  section,
  $active,
  isExpanded,
  children,
  onExpandSection,
}: PropsWithChildren<SharedSectionProps>) {
  return (
    <>
      <CategoryButton
        $active={$active}
        onMouseEnter={onExpandSection}
        onFocus={onExpandSection}
        isExpanded={isExpanded}
        tabIndex={section.standalone ? 0 : -1}
      >
        <CategoryIcon category={section.category} />
        {section.category}
      </CategoryButton>
      {children}
    </>
  );
}

/**
 * StandaloneSection is a section with no subsections, instead of expanding a drawer, the category button is clickable and takes you directly to a route.
 */
export function StandaloneSection({
  title,
  route,
  Icon,
  $active,
}: {
  title: string;
  route: string;
  Icon: (props) => ReactNode;
  $active: boolean;
}) {
  return (
    <CategoryButton as={NavLink} $active={$active} to={route}>
      <Icon />
      {title}
    </CategoryButton>
  );
}

export const rightPanelWidth = 236;
export const RightPanel: React.FC<
  PropsWithChildren<{
    isVisible: boolean;
    skipAnimation: boolean;
    id: string;
    onFocus(): void;
  }>
> = ({ isVisible, skipAnimation, id, onFocus, children }) => {
  return (
    <SlidingSidePanel
      isVisible={isVisible}
      skipAnimation={skipAnimation}
      id={id}
      onFocus={onFocus}
      panelWidth={rightPanelWidth}
      zIndex={zIndexMap.sideNavExpandedPanel}
      slideFrom="left"
      left="var(--sidenav-width)"
    >
      {children}
    </SlidingSidePanel>
  );
};

export function RightPanelHeader({
  title,
  stickyMode,
  toggleStickyMode,
  canToggleStickyMode,
}: {
  title: string;
  stickyMode: boolean;
  toggleStickyMode: () => void;
  canToggleStickyMode: boolean;
}) {
  return (
    <Flex
      py={verticalPadding}
      px={3}
      css={`
        position: relative;
      `}
    >
      <Text typography="h2" color="text.slightlyMuted">
        {title}
      </Text>
      <StickyToggleBtn
        onClick={toggleStickyMode}
        disabled={!canToggleStickyMode}
      >
        {canToggleStickyMode ? (
          <HoverTooltip tipContent={stickyMode ? 'Collapse' : 'Keep Expanded'}>
            <AnimatedArrow size={20} $isSticky={stickyMode} />
          </HoverTooltip>
        ) : (
          <AnimatedArrow size={20} $isSticky={stickyMode} />
        )}
      </StickyToggleBtn>
    </Flex>
  );
}

const StickyToggleBtn = styled(ButtonIcon)`
  position: absolute;
  right: 0;
  top: 4px;
  height: 40px;
  width: 40px;
  border-radius: ${props => props.theme.radii[2]}px;
  color: ${props => props.theme.colors.text.muted};

  &:hover:enabled,
  &:focus-visible:enabled {
    color: ${props => props.theme.colors.text.main};
  }
`;

const AnimatedArrow = styled(ArrowLineLeft)<{ $isSticky: boolean }>`
  transition: transform 0.3s ease-in-out;
  transform: ${props => (props.$isSticky ? 'none' : 'rotate(180deg)')};
`;

export const CategoryButton = styled.button<{
  $active: boolean;
  isExpanded?: boolean;
}>`
  min-height: 60px;
  min-width: 60px;
  cursor: pointer;
  outline: hidden;
  border: none;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  border-radius: ${props => props.theme.radii[2]}px;
  z-index: ${zIndexMap.sideNavButtons};
  display: flex;
  align-items: center;
  justify-content: center;
  gap: ${props => props.theme.space[1]}px;
  font-family: ${props => props.theme.font};

  font-size: ${props => props.theme.typography.body4.fontSize};
  font-weight: ${props => props.theme.typography.body4.fontWeight};
  letter-spacing: ${props => props.theme.typography.body4.letterSpacing};
  line-height: ${props => props.theme.typography.body4.lineHeight};
  text-decoration: none;

  ${props => getCategoryStyles(props.theme, props.$active, props.isExpanded)}
`;

export function getCategoryStyles(
  theme: Theme,
  active: boolean,
  isExpanded: boolean
) {
  if (active) {
    return css`
      color: ${theme.colors.brand};
      background: ${theme.colors.interactive.tonal.primary[0]};
      &:hover,
      &:focus-visible {
        background: ${theme.colors.interactive.tonal.primary[1]};
        color: ${theme.colors.interactive.solid.primary.default};
      }
      &:active {
        background: ${theme.colors.interactive.tonal.primary[2]};
        color: ${theme.colors.interactive.solid.primary.active};
      }
      ${isExpanded &&
      `
        background: ${theme.colors.interactive.tonal.primary[1]};
        color: ${theme.colors.interactive.solid.primary.default};
      `}
    `;
  }

  return css`
    background: transparent;
    color: ${theme.colors.text.slightlyMuted};
    &:hover,
    &:focus-visible {
      background: ${theme.colors.interactive.tonal.neutral[0]};
      color: ${theme.colors.text.main};
    }
    &:active {
      background: ${theme.colors.interactive.tonal.neutral[1]};
      color: ${theme.colors.text.main};
    }
    ${isExpanded &&
    `
      background: ${theme.colors.interactive.tonal.neutral[0]};
      color: ${theme.colors.text.main};
      `}
  `;
}

export const verticalPadding = '12px';

export function SubsectionItem({
  $active,
  to,
  exact,
  children,
  onClick,
}: {
  $active: boolean;
  to: string;
  exact: boolean;
  children: React.ReactNode;
  onClick?: (event: React.MouseEvent) => void;
}) {
  return (
    <StyledSubsectionItem
      $active={$active}
      to={to}
      exact={exact}
      tabIndex={0}
      onClick={onClick}
    >
      {children}
    </StyledSubsectionItem>
  );
}

export const StyledSubsectionItem = styled(NavLink)<{
  $active: boolean;
}>`
  display: flex;
  position: relative;
  color: ${props => props.theme.colors.text.slightlyMuted};
  text-decoration: none;
  user-select: none;
  gap: ${props => props.theme.space[2]}px;
  padding-top: ${verticalPadding};
  padding-bottom: ${verticalPadding};
  padding-left: ${props => props.theme.space[3]}px;
  padding-right: ${props => props.theme.space[3]}px;
  border-radius: ${props => props.theme.radii[2]}px;
  cursor: pointer;

  ${props => getSubsectionStyles(props.theme, props.$active)}
`;

export function getSubsectionStyles(theme: Theme, active: boolean) {
  if (active) {
    return css`
      color: ${theme.colors.brand};
      background: ${theme.colors.interactive.tonal.primary[0]};
      p {
        font-weight: 500;
      }
      &:focus-visible {
        outline: 2px solid ${theme.colors.interactive.solid.primary.default};
      }
      &:hover {
        background: ${theme.colors.interactive.tonal.primary[1]};
        color: ${theme.colors.interactive.solid.primary.default};
      }
      &:active {
        background: ${theme.colors.interactive.tonal.primary[2]};
        color: ${theme.colors.interactive.solid.primary.active};
      }
    `;
  }

  return css`
    color: ${props => props.theme.colors.text.slightlyMuted};
    &:focus-visible {
      outline: 2px solid ${theme.colors.text.muted};
    }
    &:hover {
      background: ${props => props.theme.colors.interactive.tonal.neutral[0]};
      color: ${props => props.theme.colors.text.main};
    }
    &:active {
      background: ${props => props.theme.colors.interactive.tonal.neutral[1]};
      color: ${props => props.theme.colors.text.main};
    }
  `;
}

function AGPLFooter() {
  const theme = useTheme();
  return (
    <LicenseFooter
      title="AGPL Edition"
      subText="Unofficial Version"
      infoContent={
        <>
          {/* This is an independently compiled AGPL-3.0 version of Teleport. You */}
          {/* can find the official release on{' '} */}
          This is an independently compiled AGPL-3.0 version of Teleport.
          <br />
          Visit{' '}
          <Text
            as="a"
            href="https://goteleport.com/download/?utm_source=oss&utm_medium=in-product&utm_campaign=limited-features"
            target="_blank"
            color={theme.colors.interactive.solid.accent.default}
          >
            the Downloads page
          </Text>{' '}
          for the official release.
        </>
      }
    />
  );
}

function CommunityFooter() {
  const theme = useTheme();
  return (
    <LicenseFooter
      title="Community Edition"
      subText="Limited Features"
      infoContent={
        <>
          <Text
            as="a"
            href="https://goteleport.com/signup/enterprise/?utm_source=oss&utm_medium=in-product&utm_campaign=limited-features"
            target="_blank"
            color={theme.colors.interactive.solid.accent.default}
          >
            Upgrade to Teleport Enterprise
          </Text>{' '}
          for SSO, just-in-time access requests, Access Graph, and much more!
        </>
      }
    />
  );
}

function LicenseFooter({
  title,
  subText,
  infoContent,
}: {
  title: string;
  subText: string;
  infoContent: JSX.Element;
}) {
  return (
    <StyledFooterBox py={3} px={4}>
      <Flex alignItems="center" gap={2}>
        <Text>{title}</Text>
        <IconTooltip position="right" sticky>
          {infoContent}
        </IconTooltip>
      </Flex>
      <SubText>{subText}</SubText>
    </StyledFooterBox>
  );
}

const StyledFooterBox = styled(Box)`
  line-height: 20px;
  border-top: ${props => props.theme.borders[1]}
    ${props => props.theme.colors.spotBackground[0]};
`;

const SubText = styled(Text)`
  color: ${props => props.theme.colors.text.disabled};
  font-size: ${props => props.theme.fontSizes[1]}px;
`;
