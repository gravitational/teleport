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

import { matchPath } from 'react-router';
import styled from 'styled-components';

import { Box, Flex, Text } from 'design';
import * as Icons from 'design/Icon';
import { DefaultTab } from 'gen-proto-ts/teleport/userpreferences/v1/unified_resource_preferences_pb';
import { UserPreferences } from 'gen-proto-ts/teleport/userpreferences/v1/userpreferences_pb';

import { encodeUrlQueryParams } from 'teleport/components/hooks/useUrlFiltering';
import { EncodeUrlQueryParamsProps } from 'teleport/components/hooks/useUrlFiltering/encodeUrlQueryParams';
import cfg from 'teleport/config';
import { ResourceIdKind } from 'teleport/services/agents';
import { useUser } from 'teleport/User/UserContext';
import useStickyClusterId from 'teleport/useStickyClusterId';

import { CustomNavigationSubcategory, NavigationCategory } from './categories';
import { NavigationSection, NavigationSubsection } from './Navigation';
import {
  CustomChildrenSection,
  RightPanel,
  RightPanelHeader,
  SubsectionItem,
  verticalPadding,
} from './Section';

/**
 * getResourcesSection returns a NavigationSection for resources,
 * this is used for the sake of indexing these subsections in the sidenav search.
 */
export function getResourcesSection(
  subsectionProps: GetSubsectionProps
): NavigationSection {
  return {
    category: NavigationCategory.Resources,
    subsections: getResourcesSubsections(subsectionProps),
  };
}

type GetSubsectionProps = {
  clusterId: string;
  preferences: UserPreferences;
  updatePreferences: (preferences: Partial<UserPreferences>) => Promise<void>;
  searchParams: URLSearchParams;
};

function encodeUrlQueryParamsWithTypedKinds(
  params: Omit<EncodeUrlQueryParamsProps, 'kinds'> & {
    kinds?: ResourceIdKind[];
  }
) {
  return encodeUrlQueryParams(params);
}

function getResourcesSubsections({
  clusterId,
  preferences,
  updatePreferences,
  searchParams,
}: GetSubsectionProps): NavigationSubsection[] {
  const baseRoute = cfg.getUnifiedResourcesRoute(clusterId);

  const setPinnedUserPreference = (pinnedOnly: boolean) => {
    // Return early if the current user preference already matches the pinnedOnly param provided, since nothing needs to be done.
    if (
      (pinnedOnly &&
        preferences?.unifiedResourcePreferences?.defaultTab ===
          DefaultTab.PINNED) ||
      (!pinnedOnly &&
        (preferences?.unifiedResourcePreferences?.defaultTab ===
          DefaultTab.ALL ||
          preferences?.unifiedResourcePreferences?.defaultTab ===
            DefaultTab.UNSPECIFIED))
    ) {
      return;
    }

    updatePreferences({
      ...preferences,
      unifiedResourcePreferences: {
        ...preferences?.unifiedResourcePreferences,
        defaultTab: pinnedOnly ? DefaultTab.PINNED : DefaultTab.ALL,
      },
    });
  };

  const currentKinds = searchParams
    .getAll('kinds')
    .flatMap(k => k.split(','))
    .filter(Boolean);
  const isPinnedOnly =
    preferences?.unifiedResourcePreferences?.defaultTab === DefaultTab.PINNED;

  // isKindActive returns true if we are currently filtering for only the provided kind of resource.
  const isKindActive = (kind: ResourceIdKind) => {
    // This subsection for this kind should only be marked active when it is the only kind being filtered for,
    // if there are multiple kinds then the "All Resources" button should be active.
    return currentKinds.length === 1 && currentKinds[0] === kind;
  };

  const allResourcesRoute = encodeUrlQueryParamsWithTypedKinds({
    pathname: baseRoute,
    pinnedOnly: false,
  });
  const pinnedOnlyRoute = encodeUrlQueryParamsWithTypedKinds({
    pathname: baseRoute,
    pinnedOnly: true,
  });
  const applicationsOnlyRoute = encodeUrlQueryParamsWithTypedKinds({
    pathname: baseRoute,
    kinds: ['app'],
    pinnedOnly: false,
  });
  const databasesOnlyRoute = encodeUrlQueryParamsWithTypedKinds({
    pathname: baseRoute,
    kinds: ['db'],
    pinnedOnly: false,
  });
  const desktopsOnlyRoute = encodeUrlQueryParamsWithTypedKinds({
    pathname: baseRoute,
    kinds: ['windows_desktop'],
    pinnedOnly: false,
  });
  const kubesOnlyRoute = encodeUrlQueryParamsWithTypedKinds({
    pathname: baseRoute,
    kinds: ['kube_cluster'],
    pinnedOnly: false,
  });
  const nodesOnlyRoute = encodeUrlQueryParamsWithTypedKinds({
    pathname: baseRoute,
    kinds: ['node'],
    pinnedOnly: false,
  });
  const gitOnlyRoute = encodeUrlQueryParamsWithTypedKinds({
    pathname: baseRoute,
    kinds: ['git_server'],
    pinnedOnly: false,
  });

  return [
    {
      title: 'All Resources',
      icon: Icons.Server,
      route: allResourcesRoute,
      searchableTags: ['resources', 'resources', 'all resources'],
      category: NavigationCategory.Resources,
      exact: false,
      customRouteMatchFn: currentViewRoute =>
        !!matchPath(currentViewRoute, {
          path: cfg.routes.unifiedResources,
          exact: false,
        }) &&
        !isPinnedOnly &&
        currentKinds.length !== 1,
      onClick: () => setPinnedUserPreference(false),
    },
    {
      title: 'Pinned Resources',
      icon: Icons.PushPin,
      route: pinnedOnlyRoute,
      searchableTags: ['resources', 'resources', 'pinned resources'],
      category: NavigationCategory.Resources,
      exact: false,
      customRouteMatchFn: currentViewRoute =>
        !!matchPath(currentViewRoute, {
          path: cfg.routes.unifiedResources,
          exact: false,
        }) &&
        isPinnedOnly &&
        currentKinds.length !== 1,
      onClick: () => setPinnedUserPreference(true),
    },
    {
      title: 'Applications',
      icon: Icons.Application,
      route: applicationsOnlyRoute,
      searchableTags: ['resources', 'apps', 'applications'],
      category: NavigationCategory.Resources,
      exact: false,
      customRouteMatchFn: () => isKindActive('app'),
      onClick: () => setPinnedUserPreference(false),
      subCategory: CustomNavigationSubcategory.FilteredViews,
    },
    {
      title: 'Databases',
      icon: Icons.Database,
      route: databasesOnlyRoute,
      searchableTags: ['resources', 'dbs', 'databases'],
      category: NavigationCategory.Resources,
      exact: false,
      customRouteMatchFn: () => isKindActive('db'),
      onClick: () => setPinnedUserPreference(false),
      subCategory: CustomNavigationSubcategory.FilteredViews,
    },
    {
      title: 'Desktops',
      icon: Icons.Desktop,
      route: desktopsOnlyRoute,
      searchableTags: ['resources', 'desktops', 'rdp', 'windows'],
      category: NavigationCategory.Resources,
      exact: false,
      customRouteMatchFn: () => isKindActive('windows_desktop'),
      onClick: () => setPinnedUserPreference(false),
      subCategory: CustomNavigationSubcategory.FilteredViews,
    },
    {
      title: 'Git Servers',
      icon: Icons.GitHub,
      route: gitOnlyRoute,
      searchableTags: ['resources', 'git', 'github', 'git servers'],
      category: NavigationCategory.Resources,
      exact: false,
      customRouteMatchFn: () => isKindActive('git_server'),
      onClick: () => setPinnedUserPreference(false),
      subCategory: CustomNavigationSubcategory.FilteredViews,
    },
    {
      title: 'Kubernetes',
      icon: Icons.Kubernetes,
      route: kubesOnlyRoute,
      searchableTags: ['resources', 'k8s', 'kubes', 'kubernetes'],
      category: NavigationCategory.Resources,
      exact: false,
      customRouteMatchFn: () => isKindActive('kube_cluster'),
      onClick: () => setPinnedUserPreference(false),
      subCategory: CustomNavigationSubcategory.FilteredViews,
    },
    {
      title: 'SSH Resources',
      icon: Icons.Server,
      route: nodesOnlyRoute,
      searchableTags: ['resources', 'servers', 'nodes', 'ssh resources'],
      category: NavigationCategory.Resources,
      exact: false,
      customRouteMatchFn: () => isKindActive('node'),
      onClick: () => setPinnedUserPreference(false),
      subCategory: CustomNavigationSubcategory.FilteredViews,
    },
  ];
}

export function ResourcesSection({
  expandedSection,
  previousExpandedSection,
  handleSetExpandedSection,
  currentView,
  stickyMode,
  toggleStickyMode,
  canToggleStickyMode,
}: {
  expandedSection: NavigationSection;
  previousExpandedSection: NavigationSection;
  currentView: NavigationSubsection;
  handleSetExpandedSection: (section: NavigationSection) => void;
  stickyMode: boolean;
  toggleStickyMode: () => void;
  canToggleStickyMode: boolean;
}) {
  const { clusterId } = useStickyClusterId();
  const { preferences, updatePreferences } = useUser();
  const section: NavigationSection = {
    category: NavigationCategory.Resources,
    subsections: [],
  };
  const baseRoute = cfg.getUnifiedResourcesRoute(clusterId);

  const searchParams = new URLSearchParams(location.search);

  const isExpanded = expandedSection?.category === NavigationCategory.Resources;

  const subsections = getResourcesSubsections({
    clusterId,
    preferences,
    updatePreferences,
    searchParams,
  });

  const currentViewRoute = currentView?.route;

  return (
    <CustomChildrenSection
      key="resources"
      section={section}
      $active={currentView?.route === baseRoute}
      onExpandSection={() => handleSetExpandedSection(section)}
      aria-controls={`panel-${expandedSection?.category}`}
      isExpanded={isExpanded}
    >
      <RightPanel
        isVisible={isExpanded}
        skipAnimation={!!previousExpandedSection}
        id={`panel-resources`}
        onFocus={() => handleSetExpandedSection(section)}
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
            canToggleStickyMode={canToggleStickyMode}
          />
          {subsections
            .filter(section => !section.subCategory)
            .map(section => (
              <SubsectionItem
                $active={section.customRouteMatchFn(currentViewRoute)}
                to={section.route}
                key={section.title}
                onClick={section.onClick}
                exact={section.exact}
              >
                <section.icon size={16} />
                <Text typography="body2">{section.title}</Text>
              </SubsectionItem>
            ))}

          <Divider />
          <Flex py={verticalPadding} px={3}>
            <Text typography="h3" color="text.slightlyMuted">
              Filtered Views
            </Text>
          </Flex>

          {subsections
            .filter(
              section =>
                section.subCategory ===
                CustomNavigationSubcategory.FilteredViews
            )
            .map(section => (
              <SubsectionItem
                $active={section.customRouteMatchFn(currentViewRoute)}
                to={section.route}
                key={section.title}
                onClick={section.onClick}
                exact={section.exact}
              >
                <section.icon size={16} />
                <Text typography="body2">{section.title}</Text>
              </SubsectionItem>
            ))}
        </Box>
      </RightPanel>
    </CustomChildrenSection>
  );
}

export const Divider = styled.div`
  height: 1px;
  width: 100%;
  background: ${props => props.theme.colors.interactive.tonal.neutral[1]};
  margin: ${props => props.theme.space[1]}px 0px
    ${props => props.theme.space[1]}px 0px;
`;
