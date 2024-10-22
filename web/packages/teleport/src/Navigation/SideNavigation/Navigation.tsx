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
import styled, { useTheme } from 'styled-components';
import { matchPath, useHistory } from 'react-router';
import { Text, Flex, Box } from 'design';

import { ToolTipInfo } from 'shared/components/ToolTip';

import cfg from 'teleport/config';

import { useFeatures } from 'teleport/FeaturesContext';

import {
  Section,
  RightPanel,
  SubsectionItem,
  verticalPadding,
} from './Section';
import { zIndexMap } from './zIndexMap';

import {
  NAVIGATION_CATEGORIES,
  STANDALONE_CATEGORIES,
  SidenavCategory,
} from './categories';
import { SearchSection } from './Search';

import type * as history from 'history';
import type { TeleportFeature } from 'teleport/types';

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

const PanelBackground = styled.div`
  width: 100%;
  height: 100%;
  background: ${p => p.theme.colors.levels.surface};
  position: absolute;
  top: 0;
  z-index: ${zIndexMap.sideNavContainer};
  border-right: 1px solid ${p => p.theme.colors.spotBackground[1]};
`;

/* NavigationSection is a section in the navbar, this can either be a standalone section (clickable button with no drawer), or a category with subsections shown in a drawer that expands. */
export type NavigationSection = {
  category: SidenavCategory;
  subsections?: NavigationSubsection[];
  /* standalone is whether this is a clickable nav section with no subsections/drawer. */
  standalone?: boolean;
};

/* NavigationSubsection is a subsection of a NavigationSection, these are the items listed in the drawer of a NavigationSection. */
export type NavigationSubsection = {
  category: SidenavCategory;
  title: string;
  route: string;
  exact: boolean;
  icon: (props) => JSX.Element;
  parent?: TeleportFeature;
  searchableTags?: string[];
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
  category: SidenavCategory,
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
      searchableTags: feature.navigationItem.searchableTags,
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
    searchableTags: feature.navigationItem.searchableTags,
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
        <SearchSection
          navigationSections={navSections}
          expandedSection={expandedSection}
          previousExpandedSection={previousExpandedSection}
          handleSetExpandedSection={handleSetExpandedSection}
          currentView={currentView}
        />
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
        <ToolTipInfo position="right" sticky>
          {infoContent}
        </ToolTipInfo>
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
