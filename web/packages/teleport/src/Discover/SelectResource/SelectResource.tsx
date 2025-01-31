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

import {
  useEffect,
  useMemo,
  useState,
  type ComponentPropsWithoutRef,
} from 'react';
import { useHistory, useLocation } from 'react-router';
import styled from 'styled-components';

import { Alert, Box, Flex, Link, P3, Text } from 'design';
import * as Icons from 'design/Icon';
import { NewTab } from 'design/Icon';
import { getPlatform, Platform } from 'design/platform';
import { Resource } from 'gen-proto-ts/teleport/userpreferences/v1/onboard_pb';
import { UserPreferences } from 'gen-proto-ts/teleport/userpreferences/v1/userpreferences_pb';

import AddApp from 'teleport/Apps/AddApp';
import { FeatureHeader, FeatureHeaderTitle } from 'teleport/components/Layout';
import { ToolTipNoPermBadge } from 'teleport/components/ToolTipNoPermBadge';
import cfg from 'teleport/config';
import {
  BASE_RESOURCES,
  getResourcePretitle,
} from 'teleport/Discover/SelectResource/resources';
import {
  HeaderSubtitle,
  PermissionsErrorMessage,
  ResourceKind,
} from 'teleport/Discover/Shared';
import { resourceKindToPreferredResource } from 'teleport/Discover/Shared/ResourceKind';
import { storageService } from 'teleport/services/storageService';
import { Acl, AuthType, OnboardDiscover } from 'teleport/services/user';
import { useUser } from 'teleport/User/UserContext';
import useTeleport from 'teleport/useTeleport';

import { getMarketingTermMatches } from './getMarketingTermMatches';
import { DiscoverIcon } from './icons';
import { SAML_APPLICATIONS } from './resources';
import {
  PrioritizedResources,
  SearchResource,
  type ResourceSpec,
} from './types';

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
      sortResources(
        // Apply access check to each resource.
        addHasAccessField(
          acl,
          filterResources(
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
            {resources.map((r, index) => {
              const title = r.name;
              const pretitle = getResourcePretitle(r);
              const select = () => {
                if (!r.hasAccess) {
                  return;
                }

                setShowApp(true);
                onSelect(r);
              };

              let resourceCardProps: ComponentPropsWithoutRef<
                'button' | typeof Link
              >;

              if (r.kind === ResourceKind.Application && r.isDialog) {
                resourceCardProps = {
                  onClick: select,
                  onKeyUp: (e: KeyboardEvent) => e.key === 'Enter' && select(),
                  role: 'button',
                };
              } else if (r.unguidedLink) {
                resourceCardProps = {
                  as: Link,
                  href: r.hasAccess ? r.unguidedLink : null,
                  target: '_blank',
                  style: { textDecoration: 'none' },
                  role: 'link',
                };
              } else {
                resourceCardProps = {
                  onClick: () => r.hasAccess && onSelect(r),
                  onKeyUp: (e: KeyboardEvent) => {
                    if (e.key === 'Enter' && r.hasAccess) {
                      onSelect(r);
                    }
                  },
                  role: 'button',
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
                  aria-label={`${pretitle} ${title}`}
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
                  <Flex px={2} alignItems="center" height="48px">
                    <Flex mr={3} justifyContent="center" width="24px">
                      <DiscoverIcon name={r.icon} />
                    </Flex>
                    <Box>
                      {pretitle && (
                        <Text typography="body3" color="text.slightlyMuted">
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

                  {r.unguidedLink && r.hasAccess ? (
                    <NewTabInCorner color="text.muted" size={18} />
                  ) : null}
                </ResourceCard>
              );
            })}
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
    case ResourceKind.ConnectMyComputer:
      // This is probably already true since without this permission the user wouldn't be able to
      // add any other resource, but let's just leave it for completeness sake.
      return acl.tokens.create;
    default:
      return false;
  }
}

function sortResourcesByKind(
  resourceKind: SearchResource,
  resources: ResourceSpec[]
) {
  let sorted: ResourceSpec[] = [];
  switch (resourceKind) {
    case SearchResource.SERVER:
      sorted = [
        ...resources.filter(r => r.kind === ResourceKind.Server),
        ...resources.filter(r => r.kind !== ResourceKind.Server),
      ];
      break;
    case SearchResource.APPLICATION:
      sorted = [
        ...resources.filter(r => r.kind === ResourceKind.Application),
        ...resources.filter(r => r.kind !== ResourceKind.Application),
      ];
      break;
    case SearchResource.DATABASE:
      sorted = [
        ...resources.filter(r => r.kind === ResourceKind.Database),
        ...resources.filter(r => r.kind !== ResourceKind.Database),
      ];
      break;
    case SearchResource.DESKTOP:
      sorted = [
        ...resources.filter(r => r.kind === ResourceKind.Desktop),
        ...resources.filter(r => r.kind !== ResourceKind.Desktop),
      ];
      break;
    case SearchResource.KUBERNETES:
      sorted = [
        ...resources.filter(r => r.kind === ResourceKind.Kubernetes),
        ...resources.filter(r => r.kind !== ResourceKind.Kubernetes),
      ];
      break;
  }
  return sorted;
}

const aBeforeB = -1;
const aAfterB = 1;
const aEqualsB = 0;

/**
 * Evaluates the predicate and prioritizes the element matching the predicate over the element that
 * doesn't.
 *
 * @example
 * comparePredicate({color: 'green'}, {color: 'red'}, (el) => el.color === 'green') // => -1 (a before b)
 * comparePredicate({color: 'red'}, {color: 'green'}, (el) => el.color === 'green') // => 1  (a after  b)
 * comparePredicate({color: 'blue'}, {color: 'pink'}, (el) => el.color === 'green') // => 0  (both are equal)
 */
function comparePredicate<ElementType>(
  a: ElementType,
  b: ElementType,
  predicate: (resource: ElementType) => boolean
): -1 | 0 | 1 {
  const aMatches = predicate(a);
  const bMatches = predicate(b);

  if (aMatches && !bMatches) {
    return aBeforeB;
  }

  if (bMatches && !aMatches) {
    return aAfterB;
  }

  return aEqualsB;
}

export function sortResources(
  resources: ResourceSpec[],
  preferences: UserPreferences,
  onboardDiscover: OnboardDiscover | undefined
) {
  const { preferredResources, hasPreferredResources } =
    getPrioritizedResources(preferences);
  const platform = getPlatform();

  const sortedResources = [...resources];
  const accessible = sortedResources.filter(r => r.hasAccess);
  const restricted = sortedResources.filter(r => !r.hasAccess);

  const hasNoResources = onboardDiscover && !onboardDiscover.hasResource;
  const prefersServers =
    hasPreferredResources &&
    preferredResources.includes(
      resourceKindToPreferredResource(ResourceKind.Server)
    );
  const prefersServersOrNoPreferences =
    prefersServers || !hasPreferredResources;
  const shouldShowConnectMyComputerFirst =
    hasNoResources &&
    prefersServersOrNoPreferences &&
    isConnectMyComputerAvailable(accessible);

  // Sort accessible resources by:
  // 1. os
  // 2. preferred
  // 3. guided
  // 4. alphabetically
  //
  // When available on the given platform, Connect My Computer is put either as the first resource
  // if the user has no resources, otherwise it's at the end of the guided group.
  accessible.sort((a, b) => {
    const compareAB = (predicate: (r: ResourceSpec) => boolean) =>
      comparePredicate(a, b, predicate);
    const areBothGuided = !a.unguidedLink && !b.unguidedLink;

    // Special cases for Connect My Computer.
    // Show Connect My Computer tile as the first resource.
    if (shouldShowConnectMyComputerFirst) {
      const prioritizeConnectMyComputer = compareAB(
        r => r.kind === ResourceKind.ConnectMyComputer
      );
      if (prioritizeConnectMyComputer) {
        return prioritizeConnectMyComputer;
      }

      // Within the guided group, deprioritize server tiles of the current user platform if Connect
      // My Computer is available.
      //
      // If the user has no resources available in the cluster, we want to nudge them towards
      // Connect My Computer rather than, say, standalone macOS setup.
      //
      // Only do this if the user doesn't explicitly prefer servers. If they prefer servers, we
      // want the servers for their platform to be displayed in their usual place so that the user
      // doesn't miss that Teleport supports them.
      if (!prefersServers && areBothGuided) {
        const deprioritizeServerForUserPlatform = compareAB(
          r => !(r.kind == ResourceKind.Server && r.platform === platform)
        );
        if (deprioritizeServerForUserPlatform) {
          return deprioritizeServerForUserPlatform;
        }
      }
    } else if (areBothGuided) {
      // Show Connect My Computer tile as the last guided resource if the user already added some
      // resources or they prefer other kinds of resources than servers.
      const deprioritizeConnectMyComputer = compareAB(
        r => r.kind !== ResourceKind.ConnectMyComputer
      );
      if (deprioritizeConnectMyComputer) {
        return deprioritizeConnectMyComputer;
      }
    }

    // Display platform resources first
    const prioritizeUserPlatform = compareAB(r => r.platform === platform);
    if (prioritizeUserPlatform) {
      return prioritizeUserPlatform;
    }

    // Display preferred resources second
    if (hasPreferredResources) {
      const prioritizePreferredResource = compareAB(r =>
        preferredResources.includes(resourceKindToPreferredResource(r.kind))
      );
      if (prioritizePreferredResource) {
        return prioritizePreferredResource;
      }
    }

    // Display guided resources third
    const prioritizeGuided = compareAB(r => !r.unguidedLink);
    if (prioritizeGuided) {
      return prioritizeGuided;
    }

    // Alpha
    return a.name.localeCompare(b.name);
  });

  // Sort restricted resources alphabetically
  restricted.sort((a, b) => {
    return a.name.localeCompare(b.name);
  });

  // Sort resources that user has access to the
  // top of the list, so it is more visible to
  // the user.
  return [...accessible, ...restricted];
}

function isConnectMyComputerAvailable(
  accessibleResources: ResourceSpec[]
): boolean {
  return !!accessibleResources.find(
    resource => resource.kind === ResourceKind.ConnectMyComputer
  );
}

/**
 * Returns prioritized resources based on user preferences cluster state
 *
 * @remarks
 * A user can have preferredResources set via onboarding either from the survey (preferredResources)
 * or various query parameters (marketingParams). We sort the list by the marketingParams if available.
 * If not, we sort by preferred resource type if available.
 * We do not search.
 *
 * @param preferences - Cluster state user preferences
 * @returns PrioritizedResources which is both the resource to prioritize and a boolean value of the value
 *
 */
function getPrioritizedResources(
  preferences: UserPreferences
): PrioritizedResources {
  const marketingParams = preferences.onboard.marketingParams;

  if (marketingParams) {
    const marketingPriorities = getMarketingTermMatches(marketingParams);
    if (marketingPriorities.length > 0) {
      return {
        hasPreferredResources: true,
        preferredResources: marketingPriorities,
      };
    }
  }

  const preferredResources = preferences.onboard.preferredResources || [];

  // hasPreferredResources will be false if all resources are selected
  const maxResources = Object.keys(Resource).length / 2 - 1;
  const selectedAll = preferredResources.length === maxResources;

  return {
    preferredResources: preferredResources,
    hasPreferredResources: preferredResources.length > 0 && !selectedAll,
  };
}

export function filterResources(
  platform: Platform,
  authType: AuthType,
  resources: ResourceSpec[]
) {
  return resources.filter(resource => {
    const resourceSupportsPlatform =
      !resource.supportedPlatforms?.length ||
      resource.supportedPlatforms.includes(platform);

    const resourceSupportsAuthType =
      !resource.supportedAuthTypes?.length ||
      resource.supportedAuthTypes.includes(authType);

    return resourceSupportsPlatform && resourceSupportsAuthType;
  });
}

function addHasAccessField(
  acl: Acl,
  resources: ResourceSpec[]
): ResourceSpec[] {
  return resources.map(r => {
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

const NewTabInCorner = styled(NewTab)`
  position: absolute;
  top: ${props => props.theme.space[3]}px;
  right: ${props => props.theme.space[3]}px;
  transition: color 0.3s;
`;

const ResourceCard = styled.button<{ hasAccess?: boolean }>`
  position: relative;
  text-align: left;
  background: ${props => props.theme.colors.spotBackground[0]};
  transition: all 0.3s;

  border: none;
  border-radius: 8px;
  padding: 12px;
  color: ${props => props.theme.colors.text.main};
  line-height: inherit;
  font-size: inherit;
  font-family: inherit;
  cursor: pointer;

  opacity: ${props => (props.hasAccess ? '1' : '0.45')};

  &:focus-visible {
    outline: none;
    box-shadow: 0 0 0 3px ${props => props.theme.colors.brand};
  }

  &:hover,
  &:focus-visible {
    background: ${props => props.theme.colors.spotBackground[1]};

    ${NewTabInCorner} {
      color: ${props => props.theme.colors.text.slightlyMuted};
    }
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
  line-height: 24px;
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
