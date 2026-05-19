/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { useCallback } from 'react';
import { useHistory, useLocation } from 'react-router';

import { Alert } from 'design/Alert/Alert';
import Box from 'design/Box/Box';
import { SortType } from 'design/DataTable/types';
import { Indicator } from 'design/Indicator/Indicator';
import {
  InfoExternalTextLink,
  InfoGuideButton,
  InfoParagraph,
  ReferenceLinks,
} from 'shared/components/SlidingSidePanel/InfoGuide/InfoGuide';

import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout/Layout';
import { listWorkloadIdentities } from 'teleport/services/workloadIdentity/workloadIdentity';
import useTeleport from 'teleport/useTeleport';

import { EmptyState } from './EmptyState/EmptyState';
import { WorkloadIdetitiesList } from './List/WorkloadIdentitiesList';

export function WorkloadIdentities() {
  const history = useHistory();
  const location = useLocation<{ prevPageTokens?: readonly string[] }>();
  const queryParams = new URLSearchParams(location.search);
  const pageToken = queryParams.get('page') ?? '';
  const sortField = queryParams.get('sort_field') || 'name';
  const sortDir = queryParams.get('sort_dir') || 'ASC';
  const searchTerm = queryParams.get('search') ?? '';

  const ctx = useTeleport();
  const flags = ctx.getFeatureFlags();
  const canList = flags.listWorkloadIdentities;

  const { isPending, isFetching, isSuccess, isError, error, data } = useQuery({
    enabled: canList,
    queryKey: [
      'workload_identities',
      'list',
      pageToken,
      sortField,
      sortDir,
      searchTerm,
    ],
    queryFn: () =>
      listWorkloadIdentities({
        pageSize: 20,
        pageToken,
        sortField,
        sortDir,
        searchTerm,
      }),
    placeholderData: keepPreviousData,
    staleTime: 30_000, // Cached pages are valid for 30 seconds
  });

  const { prevPageTokens = [] } = location.state ?? {};
  const hasNextPage = !!data?.next_page_token;
  const hasPrevPage = !!pageToken;

  const handleFetchNext = useCallback(() => {
    const search = new URLSearchParams(location.search);
    search.set('page', data?.next_page_token ?? '');

    history.replace(
      {
        pathname: location.pathname,
        search: search.toString(),
      },
      {
        prevPageTokens: [...prevPageTokens, pageToken],
      }
    );
  }, [
    data?.next_page_token,
    history,
    location.pathname,
    location.search,
    pageToken,
    prevPageTokens,
  ]);

  const handleFetchPrev = useCallback(() => {
    const prevTokens = [...prevPageTokens];
    const nextToken = prevTokens.pop();

    const search = new URLSearchParams(location.search);
    search.set('page', nextToken ?? '');

    history.replace(
      {
        pathname: location.pathname,
        search: search.toString(),
      },
      {
        prevPageTokens: prevTokens,
      }
    );
  }, [history, location.pathname, location.search, prevPageTokens]);

  const sortType: SortType = {
    fieldName: sortField,
    dir: sortDir.toLowerCase() === 'desc' ? 'DESC' : 'ASC',
  };

  const handleSortChanged = useCallback(
    (sortType: SortType) => {
      const search = new URLSearchParams(location.search);
      search.set('sort_field', sortType.fieldName);
      search.set('sort_dir', sortType.dir);
      search.set('page', '');

      history.replace({
        pathname: location.pathname,
        search: search.toString(),
      });
    },
    [history, location.pathname, location.search]
  );

  const handleSearchChange = useCallback(
    (term: string) => {
      const search = new URLSearchParams(location.search);
      search.set('search', term);
      search.set('page', '');

      history.replace({
        pathname: `${location.pathname}`,
        search: search.toString(),
      });
    },
    [history, location.pathname, location.search]
  );

  const hasUnsupportedSortError = isError && isUnsupportedSortError(error);

  if (!canList) {
    return (
      <FeatureBox>
        <Alert kind="info" mt={4}>
          You do not have permission to access Workload Identities. Missing role
          permissions: <code>workload_identity.list</code>
        </Alert>
        <EmptyState />
      </FeatureBox>
    );
  }

  const isFiltering = !!queryParams.get('search');

  if (isSuccess && !data.items?.length && !isFiltering) {
    return (
      <FeatureBox>
        <EmptyState />
      </FeatureBox>
    );
  }

  return (
    <FeatureBox>
      <FeatureHeader justifyContent="space-between">
        <FeatureHeaderTitle>Workload Identities</FeatureHeaderTitle>
        <InfoGuideButton config={{ guide: <InfoGuide /> }} />
      </FeatureHeader>

      {isPending ? (
        <Box data-testid="loading" textAlign="center" m={10}>
          <Indicator />
        </Box>
      ) : undefined}

      {isError && hasUnsupportedSortError ? (
        <Alert
          kind="warning"
          primaryAction={{
            content: 'Reset sort',
            onClick: () => {
              handleSortChanged({ fieldName: 'name', dir: 'ASC' });
            },
          }}
        >
          {error.message}
        </Alert>
      ) : undefined}

      {isError && !hasUnsupportedSortError ? (
        <Alert kind="danger">{error.message}</Alert>
      ) : undefined}

      {isSuccess ? (
        <WorkloadIdetitiesList
          data={data.items ?? []}
          fetchStatus={isFetching ? 'loading' : ''}
          onFetchNext={hasNextPage ? handleFetchNext : undefined}
          onFetchPrev={hasPrevPage ? handleFetchPrev : undefined}
          sortType={sortType}
          onSortChanged={handleSortChanged}
          onSearchChange={handleSearchChange}
          searchTerm={searchTerm}
        />
      ) : undefined}
    </FeatureBox>
  );
}

const InfoGuide = () => (
  <Box>
    <InfoParagraph>
      Teleport{' '}
      <InfoExternalTextLink
        target="_blank"
        href={InfoGuideReferenceLinks.WorkloadIdentity.href}
      >
        Workload Identity
      </InfoExternalTextLink>{' '}
      securely issues short-lived cryptographic identities to workloads. It is a
      flexible foundation for workload identity across your infrastructure,
      creating a uniform way for your workloads to authenticate regardless of
      where they are running.
    </InfoParagraph>
    <InfoParagraph>
      Teleport Workload Identity is compatible with the open-source{' '}
      <InfoExternalTextLink
        target="_blank"
        href={InfoGuideReferenceLinks.WorkloadIdentity.href}
      >
        Secure Production Identity Framework For Everyone (SPIFFE)
      </InfoExternalTextLink>{' '}
      standard. This enables interoperability between workload identity
      implementations and also provides a wealth of off-the-shelf tools and SDKs
      to simplify integration with your workloads.
    </InfoParagraph>
    <ReferenceLinks links={Object.values(InfoGuideReferenceLinks)} />
  </Box>
);

const InfoGuideReferenceLinks = {
  WorkloadIdentity: {
    title: 'Workload Identity',
    href: 'https://goteleport.com/docs/machine-workload-identity/workload-identity',
  },
  Spiffe: {
    title: 'Introduction to SPIFFE',
    href: 'https://goteleport.com/docs/machine-workload-identity/workload-identity/spiffe/',
  },
  GettingStarted: {
    title: 'Getting Started with Workload Identity',
    href: 'https://goteleport.com/docs/machine-workload-identity/workload-identity/getting-started/',
  },
};

const isUnsupportedSortError = (error: Error) => {
  return !!error?.message && error.message.includes('unsupported sort');
};
