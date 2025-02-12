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

import type * as history from 'history';
import React, {
  ReactNode,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react';
import { matchPath, useHistory } from 'react-router';
import styled from 'styled-components';

import { Box, Flex } from 'design';
import { SideNavDrawerMode } from 'gen-proto-ts/teleport/userpreferences/v1/sidenav_preferences_pb';

import cfg from 'teleport/config';
import { useFeatures } from 'teleport/FeaturesContext';
import type { TeleportFeature } from 'teleport/types';
import { useUser } from 'teleport/User/UserContext';
import useStickyClusterId from 'teleport/useStickyClusterId';

import {
  CustomNavigationSubcategory,
  NAVIGATION_CATEGORIES,
  SidenavCategory,
} from './categories';
import { getResourcesSection, ResourcesSection } from './ResourcesSection';
import { SearchSection } from './Search';
import { DefaultSection, rightPanelWidth, StandaloneSection } from './Section';
import { zIndexMap } from './zIndexMap';

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
  standalone?: {
    /* Icon is the custom icon to display for a standalone section. This should only for standalone sections, as icons for categories are derived automatically by CategoryIcon */
    Icon: (props) => ReactNode;
    /* title is the custom title of a standalone section */
    title: string;
    route: string;
  };
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
  icon: (props) => ReactNode;
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

function getDashboardNavigationSections(
  features: TeleportFeature[]
): NavigationSection[] {
  const navigationSections = features
    .filter(feature => feature.showInDashboard)
    .map(feature => ({
      standalone: {
        title: feature.navigationItem.title,
        Icon: feature.navigationItem.icon,
        route: feature.navigationItem.getLink(cfg.proxyCluster),
      },
    }));

  return navigationSections;
}

function getSubsectionsForCategory(
  category: SidenavCategory,
  features: TeleportFeature[]
): NavigationSubsection[] {
  const filteredFeatures = features.filter(
    feature =>
      feature.category === category &&
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
    feature => !!feature.topMenuItem && !feature.category
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
  let feature = features
    .filter(feature => Boolean(feature.route))
    .find(feature =>
      matchPath(route.pathname, {
        path: feature.route.path,
        exact: feature.route.exact,
      })
    );

  // If this is a child feature, use its parent as the subsection instead.
  // We do this because children of features don't appear as subsections in the sidenav, but we want to highlight
  // their parent's subsection as active.
  if (feature?.parent) {
    feature = features.find(f => f instanceof feature.parent);
  }

  if (
    !feature ||
    (!feature.category && !feature.topMenuItem && !feature.navigationItem)
  ) {
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
      category: feature?.category,
    };
  }

  return {
    category: feature.category,
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

    // If we're closing the drawer as opposed to switching to a different section (value is null and isClosing is true), apply debounce.
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
  const { clusterId } = useStickyClusterId();
  const { preferences, updatePreferences } = useUser();
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
    [features, history.location]
  );

  const stickyMode = preferences.sideNavDrawerMode === SideNavDrawerMode.STICKY;

  const toggleStickyMode = () => {
    // Close the drawer right away if they're disabling sticky mode.
    if (stickyMode) {
      setIsClosing(false);
      setPreviousExpandedSection(null);
      setTargetSection(null);
    }
    updatePreferences({
      sideNavDrawerMode: stickyMode
        ? SideNavDrawerMode.COLLAPSED
        : SideNavDrawerMode.STICKY,
    });
  };

  const navSections = useMemo(() => {
    if (cfg.isDashboard) {
      return getDashboardNavigationSections(features);
    }
    return getNavigationSections(features).filter(
      section => section.subsections.length
    );
  }, [features]);

  const topMenuSection = useMemo(() => getTopMenuSection(features), [features]);

  const resourcesSection = useMemo(() => {
    const searchParams = new URLSearchParams(location.search);
    return getResourcesSection({
      clusterId,
      preferences,
      updatePreferences,
      searchParams,
    });
  }, [clusterId, preferences, updatePreferences]);

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

  const combinedSideNavSections = useMemo(
    () => [resourcesSection, ...navSections],
    [resourcesSection, navSections]
  );
  const currentPageSection = useMemo(() => {
    return combinedSideNavSections.find(
      section => section.category === currentView?.category
    );
  }, [combinedSideNavSections, currentView]);

  const collapseDrawer = useCallback(
    (closeAfterDelay = true) => {
      if (stickyMode && currentPageSection) {
        setPreviousExpandedSection(debouncedSection);
        setTargetSection(currentPageSection);
      } else {
        setIsClosing(closeAfterDelay);
        setPreviousExpandedSection(null);
        setTargetSection(null);
      }
    },
    [currentPageSection, stickyMode, debouncedSection]
  );

  useEffect(() => {
    // Whenever the user changes page, if stickyMode is enabled and the page is part of the sidenav, the drawer should be expanded with the current page section.
    if (!stickyMode) {
      return;
    }

    // If the page is not part of the sidenav, such as Account Settings, curentPageSection will be undefined, and the drawer should be collapsed.
    if (currentPageSection) {
      // If there is already an expanded section set, don't change it.
      if (debouncedSection) {
        return;
      }
      handleSetExpandedSection(currentPageSection);
    } else {
      collapseDrawer(false);
    }
  }, [currentPageSection]);

  // Handler for clicking nav items.
  const onNavigationItemClick = useCallback(() => {
    // Clear any existing timeout
    if (navigationTimeoutRef.current) {
      clearTimeout(navigationTimeoutRef.current);
    }

    if (!stickyMode) {
      // Add a small delay to the close to allow the user to see some feedback (see the section they clicked become active).
      navigationTimeoutRef.current = setTimeout(() => {
        collapseDrawer(false);
      }, 150);
    }
  }, [collapseDrawer]);

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
    <Container
      as="nav"
      onMouseLeave={() => collapseDrawer()}
      onKeyUp={e => e.key === 'Escape' && collapseDrawer(false)}
      onBlur={(event: React.FocusEvent<HTMLDivElement, Element>) => {
        if (!event.currentTarget.contains(event.relatedTarget)) {
          collapseDrawer();
        }
      }}
      className={
        stickyMode &&
        currentPageSection &&
        !!debouncedSection &&
        !debouncedSection.standalone
          ? 'sticky-mode'
          : ''
      }
    >
      <SideNavContainer>
        <PanelBackground />
        {!cfg.isDashboard && (
          <>
            <SearchSection
              navigationSections={[...combinedSideNavSections, topMenuSection]}
              expandedSection={debouncedSection}
              previousExpandedSection={previousExpandedSection}
              handleSetExpandedSection={handleSetExpandedSection}
              currentView={currentView}
              stickyMode={stickyMode}
              toggleStickyMode={toggleStickyMode}
              canToggleStickyMode={!!currentPageSection}
            />
            <ResourcesSection
              expandedSection={debouncedSection}
              previousExpandedSection={previousExpandedSection}
              handleSetExpandedSection={handleSetExpandedSection}
              currentView={currentView}
              stickyMode={stickyMode}
              toggleStickyMode={toggleStickyMode}
              canToggleStickyMode={!!currentPageSection}
            />
          </>
        )}
        {navSections.map(section => {
          if (section.standalone) {
            return (
              <StandaloneSection
                key={section.standalone.route}
                title={section.standalone.title}
                route={section.standalone.route}
                Icon={section.standalone.Icon}
                $active={section.standalone.route === currentView?.route}
              />
            );
          }

          const isExpanded =
            !!debouncedSection &&
            !debouncedSection.standalone &&
            section.category === debouncedSection?.category;

          return (
            <React.Fragment key={section.category}>
              {section.category === 'Add New' && <Divider />}
              <DefaultSection
                key={section.category}
                section={section}
                currentView={currentView}
                previousExpandedSection={previousExpandedSection}
                onExpandSection={() => handleSetExpandedSection(section)}
                currentPageSection={currentPageSection}
                stickyMode={stickyMode}
                toggleStickyMode={toggleStickyMode}
                $active={section.category === currentView?.category}
                aria-controls={`panel-${debouncedSection?.category}`}
                onNavigationItemClick={onNavigationItemClick}
                isExpanded={isExpanded}
              />
            </React.Fragment>
          );
        })}
      </SideNavContainer>
    </Container>
  );
}

const Container = styled(Box)`
  position: relative;
  width: var(--sidenav-width);
  z-index: ${zIndexMap.sideNavContainer};

  &.sticky-mode {
    margin-right: ${rightPanelWidth}px;
  }
`;

const Divider = styled.div`
  z-index: ${zIndexMap.sideNavButtons};
  height: 1px;
  background: ${props => props.theme.colors.interactive.tonal.neutral[1]};
  width: 60px;
`;
