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

import { Alert, Box, Flex, Link, P3, Text } from 'design';
import * as Icons from 'design/Icon';
import { getPlatform } from 'design/platform';

import AddApp from 'teleport/Apps/AddApp';
import { FeatureHeader, FeatureHeaderTitle } from 'teleport/components/Layout';
import cfg from 'teleport/config';
import { BASE_RESOURCES } from 'teleport/Discover/SelectResource/resources';
import { HeaderSubtitle } from 'teleport/Discover/Shared';
import { storageService } from 'teleport/services/storageService';
import { useUser } from 'teleport/User/UserContext';
import useTeleport from 'teleport/useTeleport';

import { SAML_APPLICATIONS } from './resources';
import { Tile } from './Tile';
import { SearchResource, type ResourceSpec } from './types';
import { addHasAccessField } from './utils/checkAccess';
import { filterBySupportedPlatformsAndAuthTypes } from './utils/filters';
import { sortResourcesByKind, sortResourcesByPreferences } from './utils/sort';

interface SelectResourceProps {
  onSelect: (resource: ResourceSpec) => void;
}

type UrlLocationState = {
  entity: SearchResource; // entity takes precedence over search keywords
  searchKeywords: string;
};

function getDefaultResources(
  includeEnterpriseResources: boolean
): ResourceSpec[] {
  const RESOURCES = includeEnterpriseResources
    ? [...BASE_RESOURCES, ...SAML_APPLICATIONS]
    : BASE_RESOURCES;
  return RESOURCES;
}

export function SelectResource({ onSelect }: SelectResourceProps) {
  const ctx = useTeleport();
  const location = useLocation<UrlLocationState>();
  const history = useHistory();
  const { preferences } = useUser();

  const [search, setSearch] = useState('');
  const { acl, authType } = ctx.storeUser.state;
  const platform = getPlatform();
  const defaultResources: ResourceSpec[] = useMemo(
    () =>
      sortResourcesByPreferences(
        // Apply access check to each resource.
        addHasAccessField(
          acl,
          filterBySupportedPlatformsAndAuthTypes(
            platform,
            authType,
            getDefaultResources(cfg.isEnterprise)
          )
        ),
        preferences,
        storageService.getOnboardDiscover()
      ),
    [acl, authType, platform, preferences]
  );
  const [resources, setResources] = useState(defaultResources);

  // a user must be able to create tokens AND have access to create at least one
  // type of resource in order to be considered eligible to "add resources"
  const canAddResources =
    acl.tokens.create && defaultResources.some(r => r.hasAccess);

  const [showApp, setShowApp] = useState(false);

  function onSearch(s: string, customList?: ResourceSpec[]) {
    const list = customList || defaultResources;
    const search = s.split(' ').map(s => s.toLowerCase());
    const found = list.filter(r =>
      search.every(s => r.keywords.some(k => k.toLowerCase().includes(s)))
    );

    setResources(found);
    setSearch(s);
  }

  function onClearSearch() {
    history.replace({ state: {} }); // Clear any loc state.
    onSearch('');
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

  return (
    <Box>
      {!canAddResources && (
        <Alert kind="info" mt={5}>
          You cannot add new resources. Reach out to your Teleport administrator
          for additional permissions.
        </Alert>
      )}
      <FeatureHeader>
        <FeatureHeaderTitle>Select Resource To Add</FeatureHeaderTitle>
      </FeatureHeader>
      <HeaderSubtitle>
        Teleport can integrate into most, if not all, of your infrastructure.
        Search for what resource you want to add.
      </HeaderSubtitle>
      <Box height="90px" width="600px">
        <InputWrapper>
          <StyledInput
            placeholder="Search for a resource"
            autoFocus
            value={search}
            onChange={e => onSearch(e.target.value)}
            max={100}
          />
        </InputWrapper>
        {search && <ClearSearch onClick={onClearSearch} />}
      </Box>
      {resources && resources.length > 0 && (
        <>
          <Grid role="grid">
            {resources.map((r, index) => (
              <Tile
                // TODO(kimlisa): replace with r.id in upcoming PR
                key={`${index}${r.name}${r.kind}`}
                resourceSpec={r}
                onChangeShowApp={setShowApp}
                onSelectResource={onSelect}
              />
            ))}
          </Grid>
          <P3 mt={6}>
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

const ClearSearch = ({ onClick }: { onClick(): void }) => {
  return (
    <Flex
      width="120px"
      onClick={onClick}
      alignItems="center"
      mt={1}
      css={`
        font-size: 12px;
        opacity: 0.7;

        &:hover {
          cursor: pointer;
          opacity: 1;
        }
      `}
    >
      <Box
        mr={1}
        ml={1}
        width="18px"
        height="18px"
        borderRadius="4px"
        textAlign="center"
        css={`
          background: ${props => props.theme.colors.error.main};
        `}
      >
        <Icons.Cross size="small" />
      </Box>
      <Text>Clear search</Text>
    </Flex>
  );
};

const Grid = styled.div`
  display: grid;
  grid-template-columns: repeat(auto-fill, 320px);
  column-gap: 10px;
  row-gap: 15px;
`;

const InputWrapper = styled.div`
  border-radius: 200px;
  height: 40px;
  border: 1px solid ${props => props.theme.colors.spotBackground[2]};
  transition: all 0.1s;

  &:hover,
  &:focus-within,
  &:active {
    background: ${props => props.theme.colors.spotBackground[0]};
  }
`;

const StyledInput = styled.input`
  border: none;
  outline: none;
  box-sizing: border-box;
  height: 100%;
  width: 100%;
  transition: all 0.2s;
  color: ${props => props.theme.colors.text.main};
  background: transparent;
  margin-right: ${props => props.theme.space[3]}px;
  margin-bottom: ${props => props.theme.space[2]}px;
  padding: ${props => props.theme.space[3]}px;
`;
