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
import { Mark } from 'design/Mark/Mark';
import {
  InfoExternalTextLink,
  InfoGuideButton,
  InfoParagraph,
  ReferenceLinks,
} from 'shared/components/SlidingSidePanel/InfoGuide/InfoGuide';

import { EmptyState } from 'teleport/Bots/List/EmptyState/EmptyState';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout/Layout';
import cfg from 'teleport/config';
import { listBotInstances } from 'teleport/services/bot/bot';
import { BotInstanceSummary } from 'teleport/services/bot/types';
import useTeleport from 'teleport/useTeleport';

import { BotInstancesList } from './List/BotInstancesList';

export function BotInstances() {
  const history = useHistory();
  const location = useLocation<{ prevPageTokens?: readonly string[] }>();
  const queryParams = new URLSearchParams(location.search);
  const pageToken = queryParams.get('page') ?? '';
  const searchTerm = queryParams.get('search') ?? '';
  const query = queryParams.get('query') ?? '';
  const sortField = queryParams.get('sort_field') || 'active_at_latest';
  const sortDir = queryParams.get('sort_dir') || 'DESC';

  const ctx = useTeleport();
  const flags = ctx.getFeatureFlags();
  const canListInstances = flags.listBotInstances;

  const { isPending, isFetching, isSuccess, isError, error, data } = useQuery({
    enabled: canListInstances,
    queryKey: [
      'bot_instances',
      'list',
      searchTerm,
      query,
      pageToken,
      sortField,
      sortDir,
    ],
    queryFn: ({ signal }) =>
      listBotInstances(
        {
          pageSize: 20,
          pageToken,
          searchTerm,
          query,
          sortField,
          sortDir,
        },
        signal
      ),
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

  const onItemSelected = useCallback(
    (item: BotInstanceSummary) => {
      history.push(
        cfg.getBotInstanceDetailsRoute({
          botName: item.bot_name,
          instanceId: item.instance_id,
        })
      );
    },
    [history]
  );

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

  const hasUnsupportedSortError = isUnsupportedSortError(error);

  if (!canListInstances) {
    return (
      <FeatureBox>
        <Alert kind="info" mt={4}>
          You do not have permission to access Bot instances. Missing role
          permissions: <code>bot_instance.list</code>
        </Alert>
        <EmptyState />
      </FeatureBox>
    );
  }

  return (
    <FeatureBox>
      <FeatureHeader justifyContent="space-between">
        <FeatureHeaderTitle>Bot instances</FeatureHeaderTitle>
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
              handleSortChanged({ fieldName: 'bot_name', dir: 'ASC' });
            },
          }}
        >
          {`Error: ${error.message}`}
        </Alert>
      ) : undefined}

      {isError && !hasUnsupportedSortError ? (
        <Alert kind="danger">{`Error: ${error.message}`}</Alert>
      ) : undefined}

      {isSuccess ? (
        <BotInstancesList
          data={data.bot_instances}
          fetchStatus={isFetching ? 'loading' : ''}
          onFetchNext={hasNextPage ? handleFetchNext : undefined}
          onFetchPrev={hasPrevPage ? handleFetchPrev : undefined}
          onSearchChange={handleSearchChange}
          searchTerm={searchTerm}
          onItemSelected={onItemSelected}
          sortType={sortType}
          onSortChanged={handleSortChanged}
        />
      ) : undefined}
    </FeatureBox>
  );
}

const InfoGuide = () => (
  <Box>
    <InfoParagraph>
      A{' '}
      <InfoExternalTextLink
        target="_blank"
        href={InfoGuideReferenceLinks.BotInstances.href}
      >
        Bot Instance
      </InfoExternalTextLink>{' '}
      identifies a single lineage of{' '}
      <InfoExternalTextLink
        target="_blank"
        href={InfoGuideReferenceLinks.Bots.href}
      >
        bot
      </InfoExternalTextLink>{' '}
      identities, even through certificate renewals and rejoins. When the{' '}
      <Mark>tbot</Mark> client first authenticates to a cluster, a Bot Instance
      is generated and its UUID is embedded in the returned client identity.
    </InfoParagraph>
    <InfoParagraph>
      Bot Instances track a variety of information about <Mark>tbot</Mark>{' '}
      instances, including regular heartbeats which include basic information
      about the <Mark>tbot</Mark> host, like its architecture and OS version.
    </InfoParagraph>
    <InfoParagraph>
      {' '}
      Bot Instances have a relatively short lifespan and are set to expire after
      the most recent identity issued for that instance will expire. If the{' '}
      <Mark>tbot</Mark> client associated with a particular Bot Instance renews
      or rejoins, the expiration of the bot instance is reset. This is designed
      to allow users to list Bot Instances for an accurate view of the number of
      active <Mark>tbot</Mark> clients interacting with their Teleport cluster.
    </InfoParagraph>
    <ReferenceLinks links={Object.values(InfoGuideReferenceLinks)} />
  </Box>
);

const InfoGuideReferenceLinks = {
  BotInstances: {
    title: 'What are Bot instances',
    href: 'https://goteleport.com/docs/enroll-resources/machine-id/introduction/#bot-instances',
  },
  Bots: {
    title: 'What are Bots',
    href: 'https://goteleport.com/docs/enroll-resources/machine-id/introduction/#bots',
  },
  Tctl: {
    title: 'Use tctl to manage bot instances',
    href: 'https://goteleport.com/docs/reference/cli/tctl/#tctl-bots-instances-add',
  },
};

const isUnsupportedSortError = (error: Error | null | undefined) => {
  return error?.message && error.message.includes('unsupported sort');
};
