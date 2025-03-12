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

import { useEffect, useMemo, useState } from 'react';
import { useHistory, useLocation } from 'react-router';
import styled from 'styled-components';

import { Alert, Box, Flex, H3, Link, P3, Text } from 'design';
import { Danger } from 'design/Alert';
import InputSearch from 'design/DataTable/InputSearch';
import { Magnifier } from 'design/Icon';
import { getPlatform } from 'design/platform';
import { MultiselectMenu } from 'shared/components/Controls/MultiselectMenu';
import { PinningSupport } from 'shared/components/UnifiedResources';
import { useAsync } from 'shared/hooks/useAsync';

import AddApp from 'teleport/Apps/AddApp';
import { FeatureHeaderTitle } from 'teleport/components/Layout';
import cfg from 'teleport/config';
import { BASE_RESOURCES } from 'teleport/Discover/SelectResource/resources/resources';
import { storageService } from 'teleport/services/storageService';
import { DiscoverGuideId } from 'teleport/services/userPreferences/discoverPreference';
import { useUser } from 'teleport/User/UserContext';
import useTeleport from 'teleport/useTeleport';

import { TextIcon } from '../Shared';
import { SelectResourceSpec } from './resources';
import { SAML_APPLICATIONS } from './resources/resourcesE';
import { Tile } from './Tile';
import { SearchResource } from './types';
import { addHasAccessField } from './utils/checkAccess';
import {
  filterBySupportedPlatformsAndAuthTypes,
  filterResources,
  Filters,
  hostingPlatformOptions,
  resourceTypeOptions,
} from './utils/filters';
import {
  DiscoverResourcePreference,
  getDefaultPins,
  getPins,
} from './utils/pins';
import { sortResourcesByKind, sortResourcesByPreferences } from './utils/sort';

interface SelectResourceProps {
  onSelect: (resource: SelectResourceSpec) => void;
}

type UrlLocationState = {
  entity: SearchResource; // entity takes precedence over search keywords
  searchKeywords: string;
};

function getDefaultResources(
  includeEnterpriseResources: boolean
): SelectResourceSpec[] {
  const RESOURCES = includeEnterpriseResources
    ? [...BASE_RESOURCES, ...SAML_APPLICATIONS]
    : BASE_RESOURCES;
  return RESOURCES;
}

export function SelectResource({ onSelect }: SelectResourceProps) {
  const ctx = useTeleport();
  const location = useLocation<UrlLocationState>();
  const history = useHistory();
  const { preferences, updateDiscoverResourcePreferences } = useUser();

  const [filters, setFilters] = useState<Filters>({
    resourceTypes: [],
    hostingPlatforms: [],
  });
  const [updateDiscoverPreferenceAttempt, updateDiscoverPreference] = useAsync(
    async (newPref: DiscoverResourcePreference) => {
      await updateDiscoverResourcePreferences(newPref);
    }
  );

  const [search, setSearch] = useState('');
  const { acl, authType } = ctx.storeUser.state;
  const platform = getPlatform();

  /**
   * defaultResources does initial processing of all resource guides that will
   * be used as base for dynamic filtering and determining default pins:
   *   - sets the "hasAccess" field (checks user perms)
   *   - sets the "pinned" field (checks user discover resource preference)
   *   - filters out guides that are not supported by users
   *     platform and auth settings (eg: "Connect My Computer" guide
   *     has limited support for platforms and auth settings)
   *   - sorts resources where preferred resources are at top of the list
   *     (certain cloud editions renders a questionaire for new users asking
   *     for their interest in resources)
   */
  const defaultResources: SelectResourceSpec[] = useMemo(() => {
    const withHasAccessFieldResources = addHasAccessField(
      acl,
      filterBySupportedPlatformsAndAuthTypes(
        platform,
        authType,
        getDefaultResources(cfg.isEnterprise)
      )
    );

    return sortResourcesByPreferences(
      withHasAccessFieldResources,
      preferences,
      storageService.getOnboardDiscover()
    );
  }, [acl, authType, platform, preferences]);

  const [resources, setResources] = useState(defaultResources);

  const filteredResources = useMemo(
    () => filterResources(resources, filters),
    [filters, resources]
  );

  // a user must be able to create tokens AND have access to create at least one
  // type of resource in order to be considered eligible to "add resources"
  const canAddResources =
    acl.tokens.create && defaultResources.some(r => r.hasAccess);

  const [showApp, setShowApp] = useState(false);

  function onSearch(s: string, customList?: SelectResourceSpec[]) {
    const list = customList || defaultResources;
    if (s == '') {
      history.replace({ state: {} }); // Clear any loc state.
      setResources(list);
      setSearch(s);
      return;
    }
    const search = s.split(' ').map(s => s.toLowerCase());
    const found = list.filter(r =>
      search.every(s => r.keywords.some(k => k.toLowerCase().includes(s)))
    );

    setResources(found);
    setSearch(s);
  }

  useEffect(() => {
    // A user can come to this screen by clicking on
    // a `add <specific-resource-type>` button.
    // We sort the list by the specified resource type,
    // and then apply a search filter to it to reduce
    // the amount of results.
    // We don't do this if the resource type is `unified_resource`,
    // since we want to show all resources.
    // TODO(bl-nero): remove this once the localstorage setting to disable unified resources is removed.
    const resourceKindSpecifiedByUrlLoc = location.state?.entity;
    if (
      resourceKindSpecifiedByUrlLoc &&
      resourceKindSpecifiedByUrlLoc !== SearchResource.UNIFIED_RESOURCE
    ) {
      const sortedResourcesByKind = sortResourcesByKind(
        resourceKindSpecifiedByUrlLoc,
        defaultResources
      );
      onSearch(resourceKindSpecifiedByUrlLoc, sortedResourcesByKind);
      return;
    }

    const searchKeywordSpecifiedByUrlLoc = location.state?.searchKeywords;
    if (searchKeywordSpecifiedByUrlLoc) {
      onSearch(searchKeywordSpecifiedByUrlLoc, defaultResources);
      return;
    }

    setResources(defaultResources);
    // Processing of the lists should only happen once on init.
    // User perms remain static and URL loc state does not change.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function updatePinnedGuides(guideId: DiscoverGuideId) {
    const { discoverResourcePreferences } = preferences;

    let previousPins = discoverResourcePreferences?.discoverGuide?.pinned || [];

    if (!discoverResourcePreferences?.discoverGuide) {
      previousPins = getDefaultPins(defaultResources);
    }

    // Toggles pins.
    let latestPins: string[];
    if (previousPins.includes(guideId)) {
      latestPins = previousPins.filter(p => p !== guideId);
    } else {
      latestPins = [...previousPins, guideId];
    }

    const newPreferences: DiscoverResourcePreference = {
      discoverResourcePreferences: {
        discoverGuide: { pinned: latestPins },
      },
    };

    updateDiscoverPreference(newPreferences);
  }

  // TODO(kimlisa): DELETE IN 19.0 - only remove the check for "NotSupported".
  let pinningSupport = preferences.discoverResourcePreferences
    ? PinningSupport.Supported
    : PinningSupport.NotSupported;

  if (updateDiscoverPreferenceAttempt.status === 'processing') {
    pinningSupport = PinningSupport.Disabled;
  }

  const pins = getPins(preferences);
  let pinnedGuides: SelectResourceSpec[] = [];
  if (pins.length > 0) {
    pinnedGuides = filteredResources.filter(r => pins.includes(r.id));
  }

  return (
    <Box>
      {!canAddResources && (
        <Alert kind="info" mt={5}>
          You cannot add new resources. Reach out to your Teleport administrator
          for additional permissions.
        </Alert>
      )}
      {updateDiscoverPreferenceAttempt.status === 'error' && (
        <Danger mt={5} details={updateDiscoverPreferenceAttempt.statusText}>
          Could not update pinned resources
        </Danger>
      )}
      <Box my={3}>
        <FeatureHeaderTitle>Enroll a New Resource</FeatureHeaderTitle>
        <Text>
          Teleport can integrate with most, if not all, of your infrastructure.
          Search below for resources you want to add.
        </Text>
      </Box>
      <Flex gap={3} justifyContent="space-between">
        <Box mb={3} width="600px">
          <InputSearch
            searchValue={search}
            setSearchValue={onSearch}
            placeholder="Search for a resource"
            autoFocus
          />
        </Box>
      </Flex>
      <Flex gap={3} mb={3}>
        <MultiselectMenu
          options={resourceTypeOptions}
          onChange={resourceTypes => setFilters({ ...filters, resourceTypes })}
          selected={filters.resourceTypes || []}
          label="Resource Type"
          tooltip="Filter by resource type"
        />
        <MultiselectMenu
          options={hostingPlatformOptions}
          onChange={hostingPlatforms =>
            setFilters({ ...filters, hostingPlatforms })
          }
          selected={filters.hostingPlatforms || []}
          label="Hosting Platform"
          tooltip="Filter by hosting platform"
        />
      </Flex>
      {!filteredResources.length && (
        <TextIcon>
          <Magnifier size="small" />
          No results found
        </TextIcon>
      )}
      {pinnedGuides.length > 0 && (
        <Box mb={4}>
          <H3 mb={3}>Pinned</H3>
          <Grid role="grid" pinnedSection={true}>
            {pinnedGuides.map(r => (
              <Tile
                key={r.id}
                resourceSpec={r}
                size="large"
                onChangeShowApp={setShowApp}
                onSelectResource={onSelect}
                pinningSupport={pinningSupport}
                onChangePin={updatePinnedGuides}
                isPinned={true}
              />
            ))}
          </Grid>
        </Box>
      )}
      {filteredResources.length > 0 && (
        <>
          {pinnedGuides.length > 0 && <H3 mb={3}>All Resources</H3>}
          <Grid role="grid">
            {filteredResources.map(r => (
              <Tile
                key={r.id}
                resourceSpec={r}
                onChangeShowApp={setShowApp}
                onSelectResource={onSelect}
                pinningSupport={pinningSupport}
                onChangePin={updatePinnedGuides}
                isPinned={pins.includes(r.id)}
              />
            ))}
          </Grid>
          <P3 my={6}>
            Looking for something else?{' '}
            <Link
              href="https://github.com/gravitational/teleport/issues/new?assignees=&labels=feature-request&template=feature_request.md"
              target="_blank"
              ml={2}
            >
              Request a feature
            </Link>
          </P3>
        </>
      )}
      {showApp && <AddApp onClose={() => setShowApp(false)} />}
    </Box>
  );
}

const Grid = styled.div<{ pinnedSection?: boolean }>`
  display: grid;
  grid-template-columns: repeat(
    auto-fill,
    ${p => (p.pinnedSection ? '250px' : '320px')}
  );
  column-gap: 10px;
  row-gap: 15px;
`;
