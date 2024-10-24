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

import React, { useState, useCallback, useEffect, useRef } from 'react';
import styled, { useTheme } from 'styled-components';
import { matchPath, useHistory } from 'react-router';
import { Text, Flex, Box, P2 } from 'design';

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

// getNavSubsectionForRoute returns the sidenav subsection that the user is correctly on (based on route).
// Note that it is possible for this not to return anything, such as in the case where the user is on a page that isn't in the sidenav (eg. Account Settings).
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

/** useDebounce is used to add a slight delay to RightPanel transitions to prevent a section from collapsing if the user overshoots it or inadvertendly
 * travels over another section, such as while moving their mouse to the RightPanel. For better UX, we skip the debounce delay when opening a new section
 * from a closed state. */
function useDebounce<T>(value: T, delay: number): T {
  const [debouncedValue, setDebouncedValue] = useState<T>(value);
  const timeoutRef = useRef<NodeJS.Timeout>();

  useEffect(() => {
    // Clear the previous timeout.
    if (timeoutRef.current) {
      clearTimeout(timeoutRef.current);
    }

    // If the new value is null (closing) or we already have a value (transitioning), apply the debounce. Otherwise, update immediately.
    // This is to prevent an unnecessary delay when expanding the very first section.
    if (value === null || debouncedValue !== null) {
      timeoutRef.current = setTimeout(() => {
        setDebouncedValue(value);
      }, delay);
    } else {
      setDebouncedValue(value);
    }

    // Cleanup on unmount or when the value changes.
    return () => {
      if (timeoutRef.current) {
        clearTimeout(timeoutRef.current);
      }
    };
  }, [value, delay, debouncedValue]);

  return debouncedValue;
}

export function Navigation() {
  const features = useFeatures();
  const history = useHistory();
  const [targetSection, setTargetSection] = useState<NavigationSection | null>(
    null
  );
  const debouncedSection = useDebounce(targetSection, 120); // 120ms debounce
  const [previousExpandedSection, setPreviousExpandedSection] =
    useState<NavigationSection | null>();

  // currentView is the sidenav subsection that the user is correctly on (based on route).
  // Note that it is possible for the currentView to be undefined, such as in the case where the user is on a page that isn't in the sidenav (eg. Account Settings).
  const currentView = getNavSubsectionForRoute(features, history.location);

  const navSections = getNavigationSections(features).filter(
    section => section.subsections.length
  );

  const handleSetExpandedSection = useCallback(
    (section: NavigationSection) => {
      if (!section.standalone) {
        setPreviousExpandedSection(debouncedSection);
        setTargetSection(section);
      } else {
        setPreviousExpandedSection(null);
        setTargetSection(null);
      }
    },
    [debouncedSection]
  );

  const resetExpandedSection = useCallback(() => {
    setPreviousExpandedSection(null);
    setTargetSection(null);
  }, []);

  return (
    <Box
      as="nav"
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
          expandedSection={debouncedSection}
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
            aria-controls={`panel-${debouncedSection?.category}`}
            onClick={() => {
              if (section.standalone) {
                history.push(section.subsections[0].route);
              }
            }}
            isExpanded={
              !!debouncedSection &&
              !debouncedSection.standalone &&
              section.category === debouncedSection?.category
            }
          >
            <RightPanel
              isVisible={
                !!debouncedSection &&
                !debouncedSection.standalone &&
                section.category === debouncedSection?.category
              }
              skipAnimation={!!previousExpandedSection}
              id={`panel-${section.category}`}
              onFocus={() => handleSetExpandedSection(section)}
              onMouseEnter={() => handleSetExpandedSection(section)}
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
                        <P2>{section.title}</P2>
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
