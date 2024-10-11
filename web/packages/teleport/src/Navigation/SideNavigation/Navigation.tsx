/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import React, { useState, useCallback } from 'react';
import styled, { css } from 'styled-components';
import { matchPath, useHistory } from 'react-router';
import { NavLink } from 'react-router-dom';
import { Text, Flex, Box } from 'design';
import * as Icons from 'design/Icon';
import { Theme } from 'design/theme/themes/types';

import cfg from 'teleport/config';

import { useFeatures } from 'teleport/FeaturesContext';

import {
  NavigationCategory,
  NAVIGATION_CATEGORIES,
  STANDALONE_CATEGORIES,
} from './categories';
import { CategoryIcon } from './CategoryIcon';

import type * as history from 'history';

import type { TeleportFeature } from 'teleport/types';

export const zIndexMap = {
  topBar: 23,
  sideNavButtons: 22,
  sideNavContainer: 21,
  sideNavExpandedPanel: 20,
};

const SideNavContainer = styled(Flex).attrs({
  gap: 2,
  pt: 2,
  flexDirection: 'column',
  alignItems: 'center',
  justifyContent: 'start',
  bg: 'levels.surface',
})`
  height: 100vh;
  width: var(--sidenav-width);
  position: fixed;
  overflow: visible;
`;

const verticalPadding = '12px';

const rightPanelWidth = '236px';

const PanelBackground = styled.div`
  width: 100%;
  height: 100%;
  background: ${p => p.theme.colors.levels.surface};
  position: absolute;
  top: 0;
  z-index: ${zIndexMap.sideNavContainer};
  border-right: 1px solid ${p => p.theme.colors.spotBackground[1]};
`;

const RightPanel = styled(Box).attrs({ pt: 2, px: 2 })<{
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

type NavigationSection = {
  category: NavigationCategory;
  subsections: NavigationSubsection[];
  standalone?: boolean;
};

type NavigationSubsection = {
  category: NavigationCategory;
  title: string;
  route: string;
  exact: boolean;
  icon: (props) => JSX.Element;
  parent?: TeleportFeature;
};

function getNavigationSections(
  features: TeleportFeature[]
): NavigationSection[] {
  const navigationSections = NAVIGATION_CATEGORIES.map(category => ({
    category,
    subsections: getSubsectionsForCategory(category, features),
    standalone: STANDALONE_CATEGORIES.indexOf(category) !== -1,
  }));

  return navigationSections;
}

function getSubsectionsForCategory(
  category: NavigationCategory,
  features: TeleportFeature[]
): NavigationSubsection[] {
  const filteredFeatures = features.filter(
    feature =>
      feature.sideNavCategory === category &&
      !!feature.navigationItem &&
      !feature.parent
  );

  return filteredFeatures.map(feature => {
    return {
      category,
      title: feature.navigationItem.title,
      route: feature.navigationItem.getLink(cfg.proxyCluster),
      exact: feature.navigationItem.exact,
      icon: feature.navigationItem.icon,
    };
  });
}

function getNavSubsectionForRoute(
  features: TeleportFeature[],
  route: history.Location<unknown> | Location
): NavigationSubsection {
  const feature = features
    .filter(feature => Boolean(feature.route))
    .find(feature =>
      matchPath(route.pathname, {
        path: feature.route.path,
        exact: false,
      })
    );

  if (!feature || !feature.sideNavCategory) {
    return;
  }

  return {
    category: feature.sideNavCategory,
    title: feature.navigationItem.title,
    route: feature.navigationItem.getLink(cfg.proxyCluster),
    exact: feature.navigationItem.exact,
    icon: feature.navigationItem.icon,
  };
}

export function Navigation() {
  const features = useFeatures();
  const history = useHistory();
  const [expandedSection, setExpandedSection] =
    useState<NavigationSection | null>(null);
  const currentView = getNavSubsectionForRoute(features, history.location);
  const [previousExpandedSection, setPreviousExpandedSection] =
    useState<NavigationSection | null>();

  const navSections = getNavigationSections(features).filter(
    section => section.subsections.length
  );

  const handleSetExpandedSection = useCallback(
    (section: NavigationSection) => {
      if (!section.standalone) {
        setPreviousExpandedSection(expandedSection);
        setExpandedSection(section);
      } else {
        setPreviousExpandedSection(null);
        setExpandedSection(null);
      }
    },
    [expandedSection]
  );

  const resetExpandedSection = useCallback(() => {
    setPreviousExpandedSection(null);
    setExpandedSection(null);
  }, []);

  return (
    <Box
      onMouseLeave={() => resetExpandedSection()}
      onKeyUp={e => e.key === 'Escape' && resetExpandedSection()}
      onBlur={(event: React.FocusEvent<HTMLDivElement, Element>) => {
        if (!event.currentTarget.contains(event.relatedTarget)) {
          resetExpandedSection();
        }
      }}
      css={`
        position: relative;
        width: var(--sidenav-width);
        z-index: ${zIndexMap.sideNavContainer};
      `}
    >
      <SideNavContainer>
        <PanelBackground />
        {navSections.map(section => (
          <Section
            key={section.category}
            section={section}
            $active={section.category === currentView?.category}
            setExpandedSection={() => handleSetExpandedSection(section)}
            aria-controls={`panel-${expandedSection?.category}`}
            onClick={() => {
              if (section.standalone) {
                history.push(section.subsections[0].route);
              }
            }}
            isExpanded={
              !!expandedSection &&
              !expandedSection.standalone &&
              section.category === expandedSection?.category
            }
          >
            <RightPanel
              isVisible={
                !!expandedSection &&
                !expandedSection.standalone &&
                section.category === expandedSection?.category
              }
              skipAnimation={!!previousExpandedSection}
              id={`panel-${section.category}`}
              onFocus={() => handleSetExpandedSection(section)}
            >
              <Flex
                flexDirection="column"
                justifyContent="space-between"
                height="100%"
              >
                <Box
                  css={`
                    overflow-y: scroll;
                    padding: 3px;
                  `}
                >
                  <Flex py={verticalPadding} px={3}>
                    <Text typography="h2" color="text.slightlyMuted">
                      {section.category}
                    </Text>
                  </Flex>
                  {!section.standalone &&
                    section.subsections.map(section => (
                      <SubsectionItem
                        $active={currentView?.route === section.route}
                        to={section.route}
                        exact={section.exact}
                        key={section.title}
                      >
                        <section.icon size={16} />
                        <Text typography="body2">{section.title}</Text>
                      </SubsectionItem>
                    ))}
                </Box>
                {cfg.edition === 'oss' && <AGPLFooter />}
                {cfg.edition === 'community' && <CommunityFooter />}
              </Flex>
            </RightPanel>
          </Section>
        ))}
      </SideNavContainer>
    </Box>
  );
}

function SubsectionItem({
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

function Section({
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

const CategoryButton = styled.button<{ $active: boolean; isExpanded: boolean }>`
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

function getCategoryStyles(theme: Theme, active: boolean, isExpanded: boolean) {
  if (active) {
    return css`
      color: ${theme.colors.brand};
      background: ${theme.colors.interactive.tonal.primary[0].background};
      &:hover,
      &:focus-visible {
        background: ${theme.colors.interactive.tonal.primary[1].background};
        color: ${theme.colors.interactive.tonal.primary[0].text};
      }
      &:active {
        background: ${theme.colors.interactive.tonal.primary[2].background};
        color: ${theme.colors.interactive.tonal.primary[1].text};
      }
      ${isExpanded &&
      `
        background: ${theme.colors.interactive.tonal.primary[1].background};
        color: ${theme.colors.interactive.tonal.primary[0].text};
        `}
    `;
  }

  return css`
    background: transparent;
    color: ${theme.colors.text.slightlyMuted};
    &:hover,
    &:focus-visible {
      background: ${theme.colors.interactive.tonal.neutral[0].background};
      color: ${theme.colors.text.main};
    }
    &:active {
      background: ${theme.colors.interactive.tonal.neutral[1].background};
      color: ${theme.colors.text.main};
    }
    ${isExpanded &&
    `
      background: ${theme.colors.interactive.tonal.neutral[0].background};
      color: ${theme.colors.text.main};
      `}
  `;
}

function getSubsectionStyles(theme: Theme, active: boolean) {
  if (active) {
    return css`
      color: ${theme.colors.brand};
      background: ${theme.colors.interactive.tonal.primary[0].background};
      &:focus-visible {
        outline: 2px solid
          ${theme.colors.interactive.solid.primary.default.background};
      }
      &:hover {
        background: ${theme.colors.interactive.tonal.primary[1].background};
        color: ${theme.colors.interactive.tonal.primary[0].text};
      }
      &:active {
        background: ${theme.colors.interactive.tonal.primary[2].background};
        color: ${theme.colors.interactive.tonal.primary[1].text};
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
  const [opened, setOpened] = useState(false);
  return (
    <StyledFooterBox py={3} px={4} onMouseLeave={() => setOpened(false)}>
      <Flex alignItems="center" gap={2}>
        <Text>{title}</Text>
        <FooterContent onMouseEnter={() => setOpened(true)}>
          <Icons.Info size={16} />
          {opened && <TooltipContent>{infoContent}</TooltipContent>}
        </FooterContent>
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

const TooltipContent = styled(Box)`
  width: max-content;
  position: absolute;
  bottom: 0;
  left: 24px;
  padding: 12px 16px 12px 16px;
  box-shadow: ${p => p.theme.boxShadow[1]};
  background-color: ${props => props.theme.colors.tooltip.background};
  color: ${props => props.theme.colors.text.primaryInverse};
  z-index: ${zIndexMap.sideNavExpandedPanel + 1};
`;

const FooterContent = styled(Flex)`
  position: relative;
`;
