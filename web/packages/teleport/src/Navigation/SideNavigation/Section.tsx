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

import React from 'react';
import { NavLink } from 'react-router-dom';
import styled, { css } from 'styled-components';

import { Box } from 'design';
import { Theme } from 'design/theme';

import { NavigationSection } from './Navigation';
import { zIndexMap } from './zIndexMap';
import { CategoryIcon } from './CategoryIcon';

export function Section({
  section,
  $active,
  setExpandedSection,
  onClick,
  children,
  isExpanded,
}: {
  section: NavigationSection;
  $active: boolean;
  setExpandedSection: () => void;
  onClick: (event: React.MouseEvent) => void;
  isExpanded?: boolean;
  children?: JSX.Element;
}) {
  return (
    <>
      <CategoryButton
        $active={$active}
        onMouseEnter={setExpandedSection}
        onFocus={setExpandedSection}
        onClick={onClick}
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

const rightPanelWidth = '236px';

export const RightPanel = styled(Box).attrs({ pt: 2, px: 2 })<{
  isVisible: boolean;
  skipAnimation: boolean;
}>`
  position: fixed;
  left: var(--sidenav-width);
  height: 100%;
  scrollbar-gutter: auto;
  overflow: visible;
  width: ${rightPanelWidth};
  background: ${p => p.theme.colors.levels.surface};
  z-index: ${zIndexMap.sideNavExpandedPanel};
  border-right: 1px solid ${p => p.theme.colors.spotBackground[1]};

  ${props =>
    props.isVisible
      ? `
      ${props.skipAnimation ? '' : 'transition: transform .15s ease-out;'}
      transform: translateX(0);
      `
      : `
      ${props.skipAnimation ? '' : 'transition: transform .15s ease-in;'}
      transform: translateX(-100%);
      `}

  top: ${p => p.theme.topBarHeight[0]}px;
  padding-bottom: ${p => p.theme.topBarHeight[0] + p.theme.space[2]}px;
  @media screen and (min-width: ${p => p.theme.breakpoints.small}px) {
    top: ${p => p.theme.topBarHeight[1]}px;
    padding-bottom: ${p => p.theme.topBarHeight[1] + p.theme.space[2]}px;
  }
  @media screen and (min-width: ${p => p.theme.breakpoints.large}px) {
    top: ${p => p.theme.topBarHeight[2]}px;
    padding-bottom: ${p => p.theme.topBarHeight[3] + p.theme.space[2]}px;
  }
`;

export const CategoryButton = styled.button<{
  $active: boolean;
  isExpanded: boolean;
}>`
  height: 60px;
  width: 60px;
  cursor: pointer;
  outline: hidden;
  border: none;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  border-radius: ${props => props.theme.radii[2]}px;
  z-index: ${zIndexMap.sideNavButtons};

  font-size: ${props => props.theme.typography.body4.fontSize};
  font-weight: ${props => props.theme.typography.body4.fontWeight};
  letter-spacing: ${props => props.theme.typography.body4.letterSpacing};
  line-height: ${props => props.theme.typography.body4.lineHeight};

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
}: {
  $active: boolean;
  to: string;
  exact: boolean;
  children: React.ReactNode;
}) {
  return (
    <StyledSubsectionItem $active={$active} to={to} exact={exact} tabIndex={0}>
      {children}
    </StyledSubsectionItem>
  );
}

const StyledSubsectionItem = styled(NavLink)<{
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
