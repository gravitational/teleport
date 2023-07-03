/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React, { useState } from 'react';
import { useLocation, useHistory } from 'react-router';

import * as Icons from 'design/Icon';
import styled from 'styled-components';
import { Box, Flex, Text, Link } from 'design';

import useTeleport from 'teleport/useTeleport';
import { ToolTipNoPermBadge } from 'teleport/components/ToolTipNoPermBadge';
import { Acl } from 'teleport/services/user';
import {
  ResourceKind,
  Header,
  HeaderSubtitle,
  PermissionsErrorMessage,
} from 'teleport/Discover/Shared';
import {
  getResourcePretitle,
  RESOURCES,
} from 'teleport/Discover/SelectResource/resources';
import AddApp from 'teleport/Apps/AddApp';

import { icons } from './icons';

import type { ResourceSpec } from './types';
import type { AddButtonResourceKind } from 'teleport/components/AgentButtonAdd/AgentButtonAdd';

interface SelectResourceProps {
  onSelect: (resource: ResourceSpec) => void;
}

export function SelectResource({ onSelect }: SelectResourceProps) {
  const ctx = useTeleport();
  const location = useLocation<{ entity: AddButtonResourceKind }>();
  const history = useHistory();

  const [search, setSearch] = useState('');
  const [resources, setResources] = useState<ResourceSpec[]>([]);
  const [defaultResources, setDefaultResources] = useState<ResourceSpec[]>([]);
  const [showApp, setShowApp] = useState(false);

  function onSearch(s: string, customList?: ResourceSpec[]) {
    const list = customList || defaultResources;
    const splitted = s.split(' ').map(s => s.toLowerCase());
    const foundResources = list.filter(r => {
      const match = splitted.every(s => r.keywords.includes(s));
      if (match) {
        return r;
      }
    });
    setResources(foundResources);
    setSearch(s);
  }

  function onClearSearch() {
    history.replace({ state: {} }); // Clear any loc state.
    onSearch('');
  }

  React.useEffect(() => {
    // Apply access check to each resource.
    const userContext = ctx.storeUser.state;
    const { acl } = userContext;
    const updatedResources = makeResourcesWithHasAccessField(acl);

    // Sort resources that user has access to the
    // the top of the list, so it is more visible to
    // the user.
    const filteredResourcesByPerm = [
      ...updatedResources.filter(r => r.hasAccess),
      ...updatedResources.filter(r => !r.hasAccess),
    ];
    const sortedResources = sortResources(filteredResourcesByPerm);
    setDefaultResources(sortedResources);

    // A user can come to this screen by clicking on
    // a `add <specific-resource-type>` button.
    // We sort the list by the specified resource type,
    // and then apply a search filter to it to reduce
    // the amount of results.
    const resourceKindSpecifiedByUrlLoc = location.state?.entity;
    if (resourceKindSpecifiedByUrlLoc) {
      const sortedResourcesByKind = sortResourcesByKind(
        resourceKindSpecifiedByUrlLoc,
        sortedResources
      );
      onSearch(resourceKindSpecifiedByUrlLoc, sortedResourcesByKind);
    } else {
      setResources(sortedResources);
    }

    // Processing of the lists should only happen once on init.
    // User perms remain static and URL loc state does not change.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <Box mt={4}>
      <Header>Select Resource To Add</Header>
      <HeaderSubtitle>
        Teleport can integrate into most, if not all of your infrastructure.
        Search for what resource you want to add.
      </HeaderSubtitle>
      <Box height="90px" width="600px">
        <InputWrapper mb={2}>
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
      {resources.length > 0 && (
        <>
          <Grid>
            {resources.map((r, index) => {
              const title = r.name;
              const pretitle = getResourcePretitle(r);

              let resourceCardProps;
              if (r.kind === ResourceKind.Application) {
                resourceCardProps = {
                  onClick: () => {
                    if (r.hasAccess) {
                      setShowApp(true);
                      onSelect(r);
                    }
                  },
                };
              } else if (r.unguidedLink) {
                resourceCardProps = {
                  as: Link,
                  href: r.hasAccess ? r.unguidedLink : null,
                  target: '_blank',
                  style: { textDecoration: 'none' },
                };
              } else {
                resourceCardProps = {
                  onClick: () => r.hasAccess && onSelect(r),
                };
              }

              // There can be three types of click behavior with the resource cards:
              //  1) If the resource has no interactive UI flow ("unguided"),
              //     clicking on the card will take a user to our docs page
              //     on a new tab.
              //  2) If the resource is guided, we start the "flow" by
              //     taking user to the next step.
              //  3) If the resource is kind 'Application', it will render the legacy
              //     popup modal where it shows user to add app manually or automatically.
              return (
                <ResourceCard
                  data-testid={r.kind}
                  key={`${index}${pretitle}${title}`}
                  hasAccess={r.hasAccess}
                  {...resourceCardProps}
                >
                  {!r.unguidedLink && r.hasAccess && (
                    <BadgeGuided>Guided</BadgeGuided>
                  )}
                  {!r.hasAccess && (
                    <ToolTipNoPermBadge
                      children={<PermissionsErrorMessage resource={r} />}
                    />
                  )}
                  <Flex px={2} alignItems="center">
                    <Flex mr={3} justifyContent="center" width="24px">
                      {icons[r.icon]}
                    </Flex>
                    <Box>
                      {pretitle && (
                        <Text fontSize="12px" color="text.slightlyMuted">
                          {pretitle}
                        </Text>
                      )}
                      {r.unguidedLink ? (
                        <Text bold color="text.main">
                          {title}
                        </Text>
                      ) : (
                        <Text bold>{title}</Text>
                      )}
                    </Box>
                  </Flex>
                </ResourceCard>
              );
            })}
          </Grid>
          <Text mt={6} fontSize="12px">
            Looking for something else?{' '}
            <Link
              href="https://github.com/gravitational/teleport/issues/new?assignees=&labels=feature-request&template=feature_request.md"
              target="_blank"
              ml={2}
            >
              Request a feature
            </Link>
          </Text>
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
        :hover {
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
        <Icons.Close fontSize="18px" />
      </Box>
      <Text>Clear search</Text>
    </Flex>
  );
};

function checkHasAccess(acl: Acl, resourceKind: ResourceKind) {
  const basePerm = acl.tokens.create;
  if (!basePerm) {
    return false;
  }

  switch (resourceKind) {
    case ResourceKind.Application:
      return acl.appServers.read && acl.appServers.list;
    case ResourceKind.Database:
      return acl.dbServers.read && acl.dbServers.list;
    case ResourceKind.Desktop:
      return acl.desktops.read && acl.desktops.list;
    case ResourceKind.Kubernetes:
      return acl.kubeServers.read && acl.kubeServers.list;
    case ResourceKind.Server:
      return acl.nodes.list;
    case ResourceKind.SamlApplication:
      return acl.samlIdpServiceProvider.create;
    default:
      return false;
  }
}

function sortResourcesByKind(
  resourceKind: AddButtonResourceKind,
  resources: ResourceSpec[]
) {
  let sorted: ResourceSpec[] = [];
  switch (resourceKind) {
    case 'server':
      sorted = [
        ...resources.filter(r => r.kind === ResourceKind.Server),
        ...resources.filter(r => r.kind !== ResourceKind.Server),
      ];
      break;
    case 'application':
      sorted = [
        ...resources.filter(r => r.kind === ResourceKind.Application),
        ...resources.filter(r => r.kind !== ResourceKind.Application),
      ];
      break;
    case 'database':
      sorted = [
        ...resources.filter(r => r.kind === ResourceKind.Database),
        ...resources.filter(r => r.kind !== ResourceKind.Database),
      ];
      break;
    case 'desktop':
      sorted = [
        ...resources.filter(r => r.kind === ResourceKind.Desktop),
        ...resources.filter(r => r.kind !== ResourceKind.Desktop),
      ];
      break;
    case 'kubernetes':
      sorted = [
        ...resources.filter(r => r.kind === ResourceKind.Kubernetes),
        ...resources.filter(r => r.kind !== ResourceKind.Kubernetes),
      ];
      break;
  }
  return sorted;
}

// Sort the resources alphabetically and with the Guided resources listed first.
export function sortResources(resources: ResourceSpec[]) {
  const sortedResources = [...resources];
  sortedResources.sort((a, b) => {
    if (!a.unguidedLink && a.hasAccess && !b.unguidedLink && b.hasAccess) {
      return a.name.localeCompare(b.name);
    }
    if (!b.unguidedLink && b.hasAccess) {
      return 1;
    }
    if (!a.unguidedLink && a.hasAccess) {
      return -1;
    }
    return a.name.localeCompare(b.name);
  });

  return sortedResources;
}

function makeResourcesWithHasAccessField(acl: Acl): ResourceSpec[] {
  return RESOURCES.map(r => {
    const hasAccess = checkHasAccess(acl, r.kind);
    switch (r.kind) {
      case ResourceKind.Database:
        return { ...r, dbMeta: { ...r.dbMeta }, hasAccess };
      default:
        return { ...r, hasAccess };
    }
  });
}

const Grid = styled.div`
  display: grid;
  grid-template-columns: repeat(auto-fill, 320px);
  column-gap: 10px;
  row-gap: 15px;
`;

const ResourceCard = styled.div`
  display: flex;
  position: relative;
  align-items: center;
  background: ${props => props.theme.colors.spotBackground[0]};
  transition: all 0.3s;

  border-radius: 8px;
  padding: 12px 12px 12px 12px;
  color: ${props => props.theme.colors.text.main};
  cursor: pointer;
  height: 48px;

  opacity: ${props => (props.hasAccess ? '1' : '0.45')};

  :hover {
    background: ${props => props.theme.colors.spotBackground[1]};
  }
`;

const BadgeGuided = styled.div`
  position: absolute;
  background: ${props => props.theme.colors.brand};
  color: ${props => props.theme.colors.text.primaryInverse};
  padding: 0px 6px;
  border-top-right-radius: 8px;
  border-bottom-left-radius: 8px;
  top: 0px;
  right: 0px;
  font-size: 10px;
`;

const InputWrapper = styled.div`
  border-radius: 200px;
  height: 40px;
  border: 1px solid ${props => props.theme.colors.spotBackground[2]};
  &:hover,
  &:focus,
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
