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

import React, {
  useState,
  useCallback,
  useEffect,
  useRef,
  useMemo,
} from 'react';
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
  CustomNavigationSubcategory,
  NAVIGATION_CATEGORIES,
  SidenavCategory,
} from './categories';
import { SearchSection } from './Search';
import { ResourcesSection } from './ResourcesSection';

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
  category?: SidenavCategory;
  subsections?: NavigationSubsection[];
  /* standalone is whether this is a clickable nav section with no subsections/drawer. */
  standalone?: boolean;
};

/**
 * NavigationSubsection is a subsection of a NavigationSection, these are the items listed in the drawer of a NavigationSection, or if isTopMenuItem is true, in the top menu (eg. Account Settings).
 */
export type NavigationSubsection = {
  category?: SidenavCategory;
  isTopMenuItem?: boolean;
  title: string;
  route: string;
  exact: boolean;
  icon: (props) => JSX.Element;
  parent?: TeleportFeature;
  searchableTags?: string[];
  /**
   * customRouteMatchFn is a custom function for determining whether this subsection is currently active,
   * this is useful in cases where a simple base route match isn't sufficient.
   */
  customRouteMatchFn?: (currentViewRoute: string) => boolean;
  /**
   * subCategory is the subcategory (ie. subsection grouping) this subsection should be under, if applicable.
   * */
  subCategory?: CustomNavigationSubcategory;
  /**
   * onClick is custom code that can be run when clicking on the subsection.
   * Note that this is merely extra logic, and does not replace the default routing behaviour of a subsection which will navigate the user to the route.
   */
  onClick?: () => void;
};

function getNavigationSections(
  features: TeleportFeature[]
): NavigationSection[] {
  const navigationSections = NAVIGATION_CATEGORIES.map(category => ({
    category,
    subsections: getSubsectionsForCategory(category, features),
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
/**
 * getTopMenuSection returns a NavigationSection with the top menu items. This is not used in the sidenav, but will be used to make the top menu items searchable.
 */
function getTopMenuSection(features: TeleportFeature[]): NavigationSection {
  const topMenuItems = features.filter(
    feature => !!feature.topMenuItem && !feature.sideNavCategory
  );

  return {
    subsections: topMenuItems.map(feature => ({
      isTopMenuItem: true,
      title: feature.topMenuItem.title,
      route: feature.topMenuItem.getLink(cfg.proxyCluster),
      exact: feature?.route?.exact,
      icon: feature.topMenuItem.icon,
      searchableTags: feature.topMenuItem.searchableTags,
    })),
  };
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
        exact: feature.route.exact,
      })
    );

  if (!feature || (!feature.sideNavCategory && !feature.topMenuItem)) {
    return;
  }

  if (feature.topMenuItem) {
    return {
      isTopMenuItem: true,
      exact: feature.route.exact,
      title: feature.topMenuItem.title,
      route: feature.topMenuItem.getLink(cfg.proxyCluster),
      icon: feature.topMenuItem.icon,
      searchableTags: feature.topMenuItem.searchableTags,
      category: feature?.sideNavCategory,
    };
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

/**
 * useDebounceClose adds a debounce to closing drawers, this is to prevent the drawer closing if the user overshoots it, giving them a slight delay to re-enter the drawer.
 */
function useDebounceClose<T>(
  value: T | null,
  delay: number,
  isClosing: boolean
): T | null {
  const [debouncedValue, setDebouncedValue] = useState<T | null>(value);
  const timeoutRef = useRef<NodeJS.Timeout>();

  useEffect(() => {
    // Clear any existing timeout
    if (timeoutRef.current) {
      clearTimeout(timeoutRef.current);
    }

    // If we're closing the drarwer as opposed to switching to a different section (value is null and isClosing is true), apply debounce.
    if (value === null && isClosing) {
      timeoutRef.current = setTimeout(() => {
        setDebouncedValue(null);
      }, delay);
    } else {
      // For opening or any other change, update immediately.
      setDebouncedValue(value);
    }

    return () => {
      if (timeoutRef.current) {
        clearTimeout(timeoutRef.current);
      }
    };
  }, [value, delay, isClosing]);

  return debouncedValue;
}

export function Navigation() {
  const features = useFeatures();
  const history = useHistory();
  const [targetSection, setTargetSection] = useState<NavigationSection | null>(
    null
  );
  const [isClosing, setIsClosing] = useState(false);
  const debouncedSection = useDebounceClose(targetSection, 200, isClosing);
  const [previousExpandedSection, setPreviousExpandedSection] =
    useState<NavigationSection | null>();
  const navigationTimeoutRef = useRef<NodeJS.Timeout>();

  // Clear navigation timeout on unmount.
  useEffect(() => {
    return () => {
      if (navigationTimeoutRef.current) {
        clearTimeout(navigationTimeoutRef.current);
      }
    };
  }, []);
  const currentView = useMemo(
    () => getNavSubsectionForRoute(features, history.location),
    [history.location]
  );

  const navSections = getNavigationSections(features).filter(
    section => section.subsections.length
  );
  const topMenuSection = getTopMenuSection(features);

  const handleSetExpandedSection = useCallback(
    (section: NavigationSection) => {
      setIsClosing(false);
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

  const resetExpandedSection = useCallback((closeAfterDelay = true) => {
    setIsClosing(closeAfterDelay);
    setPreviousExpandedSection(null);
    setTargetSection(null);
  }, []);

  // Handler for navigation actions
  const handleNavigation = useCallback(
    (route: string) => {
      history.push(route);

      // Clear any existing timeout
      if (navigationTimeoutRef.current) {
        clearTimeout(navigationTimeoutRef.current);
      }

      // Add a small delay to the close to allow the user to see some feedback (see the section they clicked become active).
      navigationTimeoutRef.current = setTimeout(() => {
        resetExpandedSection(false);
      }, 150);
    },
    [resetExpandedSection, history]
  );

  // Hide the nav if the current feature has hideNavigation set to true.
  const hideNav = features.find(
    f =>
      f.route &&
      matchPath(history.location.pathname, {
        path: f.route.path,
        exact: f.route.exact ?? false,
      })
  )?.hideNavigation;

  if (hideNav) {
    return null;
  }

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
          navigationSections={[...navSections, topMenuSection]}
          expandedSection={debouncedSection}
          previousExpandedSection={previousExpandedSection}
          handleSetExpandedSection={handleSetExpandedSection}
          currentView={currentView}
        />
        <ResourcesSection
          expandedSection={debouncedSection}
          previousExpandedSection={previousExpandedSection}
          handleSetExpandedSection={handleSetExpandedSection}
          currentView={currentView}
        />
        {navSections.map(section => (
          <React.Fragment key={section.category}>
            {section.category === 'Add New' && <Divider />}
            <Section
              key={section.category}
              section={section}
              $active={section.category === currentView?.category}
              setExpandedSection={() => handleSetExpandedSection(section)}
              aria-controls={`panel-${debouncedSection?.category}`}
              onClick={() => {
                if (section.standalone) {
                  handleNavigation(section.subsections[0].route);
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
                      section.subsections.map(subsection => (
                        <SubsectionItem
                          $active={currentView?.route === subsection.route}
                          to={subsection.route}
                          exact={subsection.exact}
                          key={subsection.title}
                          onClick={(e: React.MouseEvent) => {
                            e.preventDefault();
                            handleNavigation(subsection.route);
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
            </Section>
          </React.Fragment>
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

const Divider = styled.div`
  z-index: ${zIndexMap.sideNavButtons};
  height: 1px;
  background: ${props => props.theme.colors.interactive.tonal.neutral[1]};
  width: 60px;
`;
