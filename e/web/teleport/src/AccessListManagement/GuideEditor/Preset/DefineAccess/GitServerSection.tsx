import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import { components } from 'react-select';

import {
  Alert,
  Box,
  Link as ExternalLink,
  Flex,
  H2,
  Indicator,
  Text,
} from 'design';
import { IconTooltip } from 'design/Tooltip';
import { FieldSelect } from 'shared/components/FieldSelect';
import { Option } from 'shared/components/Select';

import { useAccessListManagementContext } from 'e-teleport/AccessListManagement/AccessListManagementContext';
import { URLResourceFilter } from 'teleport/components/hooks/useUrlFiltering/useUrlFiltering';
import cfg from 'teleport/config';
import useTeleport from 'teleport/useTeleport';

import { getGitHubOrgs } from '../role/resources/github';
import { wildcard } from '../role/role';
import { EmptyList } from './EmptyList';
import { EmptyAccess } from './ListResourceSection/EmptyAccess';
import { getResourceKindName, getResourceRbacLink } from './shared';
import { getPredicateExpression } from './unifiedResource';

/**
 * Allows defining access to git_servers through a selector dropdown menu.
 * git_server access is defined by defining a list of allowed github org
 * names (git_server names), different from label based resources.
 *
 * If no git servers exist in cluster, users will not be able to define any
 * access to git_servers.
 *
 * If users select wildcard, instead of populating all results in the dropdown
 * menu, the unified resource component is rendered instead to utilize its
 * paging and infinite scrolling.
 */
export function GitServerSection({
  updateResourceFilters,
}: {
  updateResourceFilters(newFilters: URLResourceFilter): void;
}) {
  const teleCtx = useTeleport();
  const clusterId = cfg.proxyCluster;
  const { guideEditor } = useAccessListManagementContext();
  const { standardRoleState } = guideEditor;

  const [search, setSearch] = useState('');

  const orgs = getGitHubOrgs(
    standardRoleState.roleConditions.github_permissions
  );
  const orgOptions: Option[] = orgs.map(org => ({
    label: org,
    value: org,
  }));

  const hasDefinedAccess = orgs.length > 0;
  const selectedWildcard = orgs.includes(wildcard);

  const {
    data: gitServerOptions,
    isLoading,
    isFetching,
    isError,
    error,
    isSuccess,
    refetch,
  } = useQuery({
    queryKey: ['git-server-options', clusterId, search],
    // Keep the previous data when loading new, allows isLoading to stay
    // `false` when queryKey changes.
    placeholderData: keepPreviousData,
    queryFn: async ({ queryKey }) => {
      const [, clusterFromKey, searchFromKey] = queryKey;
      const response = await teleCtx.resourceService.fetchUnifiedResources(
        clusterFromKey,
        {
          search: searchFromKey || undefined,
          kinds: ['git_server'],
          limit: 50,
        }
      );

      let options = response.agents.flatMap(a =>
        a.kind === 'git_server'
          ? [
              {
                value: a.github.organization,
                label: a.github.organization,
              },
            ]
          : []
      );

      if (!search) {
        options = [wildCardOption, ...options];
      }

      return options;
    },
  });

  function onGitServerSelectChange(values: Option[]) {
    let orgs: string[] = values.map(p => p.value);
    // Trigger the unified resources table to re-render.
    if (orgs.includes(wildcard)) {
      orgs = orgs.filter(o => o === wildcard);
      updateResourceFilters({ query: undefined, kinds: ['git_server'] });
    } else {
      updateResourceFilters({
        query: getPredicateExpression({
          accessField: 'github_permissions',
          roleConditions: {
            ...standardRoleState.roleConditions,
            github_permissions: orgs.length ? [{ orgs }] : [],
          },
        }),
        kinds: ['git_server'],
      });
    }
    standardRoleState.updateGitHubPermissions(orgs);
  }

  const optionsContainWildcard = orgOptions.some(org => org.value === wildcard);

  const { resourceKind, byline } = getResourceKindName('github_permissions');

  const noServersInCluster =
    !search &&
    isSuccess &&
    gitServerOptions.length === 1 &&
    gitServerOptions[0].value === wildcard;

  if (noServersInCluster) {
    return <EmptyList resourceField={'github_permissions'} />;
  }

  const isErrorRetry = isError && isFetching;

  return (
    <>
      <Flex alignItems={'center'} gap={2} mt={2}>
        <H2 bold css={{ textTransform: 'capitalize' }}>
          Define {resourceKind} Access
        </H2>

        <IconTooltip sticky>
          <Text>
            Learn how{' '}
            <ExternalLink
              target="_blank"
              href={getResourceRbacLink('github_permissions')}
            >
              access to {byline}
            </ExternalLink>{' '}
            works.
          </Text>
        </IconTooltip>
      </Flex>

      <Text mt={3} mb={1}>
        Select which GitHub organizations are allowed:
      </Text>

      <FieldSelect
        data-testid={'git-server-dropdown'}
        mb={1}
        width="600px"
        menuPosition="fixed"
        placeholder="Click to select or search for GitHub organizations"
        isSearchable={!selectedWildcard}
        isMulti
        isClearable
        value={orgOptions}
        options={gitServerOptions}
        onChange={onGitServerSelectChange}
        noOptionsMessage={() => (optionsContainWildcard ? null : 'No results')}
        onInputChange={value => setSearch(value)}
        components={{
          DropdownIndicator: optionsContainWildcard
            ? null
            : components.DropdownIndicator,
        }}
        menuIsOpen={optionsContainWildcard ? false : undefined}
        elevated={true}
        isLoading={isFetching}
        isDisabled={isLoading || isErrorRetry}
      />

      {(isLoading || isErrorRetry) && (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}

      {isError && !isFetching && (
        <Alert
          mt={3}
          primaryAction={{
            content: 'Retry',
            onClick: () => {
              void refetch();
            },
          }}
        >
          {error.message}
        </Alert>
      )}

      {(!isLoading || isSuccess) && !isErrorRetry && !hasDefinedAccess && (
        <EmptyAccess resourceField={'github_permissions'} />
      )}
    </>
  );
}

const wildCardOption: Option = {
  label: 'use wildcard (*) - give access to all GitHub organizations',
  value: wildcard,
};
