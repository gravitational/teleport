import { keepPreviousData, useQuery } from '@tanstack/react-query';
import {
  format,
  formatDistanceToNowStrict,
  isBefore,
  parseISO,
} from 'date-fns';
import { useCallback } from 'react';
import { useNavigate, useLocation, Location, Link } from 'react-router';
import styled from 'styled-components';

import { Alert } from 'design/Alert/Alert';
import Box from 'design/Box/Box';
import { Cell } from 'design/DataTable';
import Table from 'design/DataTable/Table';
import { SortType } from 'design/DataTable/types';
import Flex from 'design/Flex';
import { Indicator } from 'design/Indicator/Indicator';
import Text, { H2 } from 'design/Text';
import { P } from 'design/Text/Text';
import { Toggle } from 'design/Toggle';
import { HoverTooltip } from 'design/Tooltip/HoverTooltip';
import { SortMenu } from 'shared/components/Controls/SortMenu';
import { CopyButton } from 'shared/components/CopyButton/CopyButton';

import cfg from 'e-teleport/config';
import { listBeams } from 'e-teleport/services/beams/beams';
import {
  FeatureBox,
  FeatureHeaderTitle,
} from 'teleport/components/Layout/Layout';
import useTeleport from 'teleport/useTeleport';

const DEFAULT_SORT_FIELD = 'expires';
const DEFAULT_SORT_DIR = 'ASC';

export function BeamsList() {
  const navigate = useNavigate();
  const location = useLocation() as Location<{
    prevPageTokens?: readonly string[];
  }>;
  const { storeUser } = useTeleport();

  const queryParams = new URLSearchParams(location.search);
  const pageToken = queryParams.get('page') ?? '';
  const sortField = queryParams.get('sort_field') || DEFAULT_SORT_FIELD;
  const sortDir = queryParams.get('sort_dir') || DEFAULT_SORT_DIR;
  const filterOwn = queryParams.get('own') !== 'false';
  const users = filterOwn ? [storeUser.getUsername()] : undefined;

  const ctx = useTeleport();
  const flags = ctx.getFeatureFlags();
  const canList = flags.listBeam && flags.readBeam;

  const { isPending, isFetching, isSuccess, isError, error, data } = useQuery({
    enabled: canList,
    queryKey: ['beams', 'list', pageToken, sortField, sortDir, users],
    queryFn: () =>
      listBeams({
        pageSize: 20,
        pageToken,
        sortField,
        sortDir,
        users,
      }),
    placeholderData: keepPreviousData,
    staleTime: 30_000,
  });

  const { prevPageTokens = [] } = location.state ?? {};
  const hasNextPage = !!data?.next_page_token;
  const hasPrevPage = !!pageToken;

  const handleFetchNext = useCallback(() => {
    const search = new URLSearchParams(location.search);
    if (data?.next_page_token) {
      search.set('page', data.next_page_token);
    } else {
      search.delete('page');
    }

    navigate(
      {
        pathname: location.pathname,
        search: search.toString(),
      },
      {
        replace: true,
        state: { prevPageTokens: [...prevPageTokens, pageToken] },
      }
    );
  }, [
    data?.next_page_token,
    navigate,
    location.pathname,
    location.search,
    pageToken,
    prevPageTokens,
  ]);

  const handleFetchPrev = useCallback(() => {
    const prevTokens = [...prevPageTokens];
    const nextToken = prevTokens.pop();

    const search = new URLSearchParams(location.search);
    if (nextToken) {
      search.set('page', nextToken);
    } else {
      search.delete('page');
    }

    navigate(
      {
        pathname: location.pathname,
        search: search.toString(),
      },
      {
        replace: true,
        state: { prevPageTokens: prevTokens },
      }
    );
  }, [navigate, location.pathname, location.search, prevPageTokens]);

  const sortType: SortType = {
    fieldName: sortField,
    dir: sortDir.toLowerCase() === 'desc' ? 'DESC' : 'ASC',
  };

  const handleSortChanged = useCallback(
    (sortType: SortType) => {
      const search = new URLSearchParams(location.search);
      if (sortType.fieldName === DEFAULT_SORT_FIELD) {
        search.delete('sort_field');
      } else {
        search.set('sort_field', sortType.fieldName);
      }
      if (sortType.dir === DEFAULT_SORT_DIR) {
        search.delete('sort_dir');
      } else {
        search.set('sort_dir', sortType.dir);
      }
      search.delete('page');

      navigate(
        {
          pathname: location.pathname,
          search: search.toString(),
        },
        { replace: true }
      );
    },
    [navigate, location.pathname, location.search]
  );

  const handleToggleFilterOwn = useCallback(() => {
    const search = new URLSearchParams(location.search);
    if (filterOwn) {
      search.set('own', 'false');
    } else {
      search.delete('own');
    }
    search.delete('page');

    navigate(
      {
        pathname: location.pathname,
        search: search.toString(),
      },
      { replace: true }
    );
  }, [filterOwn, navigate, location]);

  if (!canList) {
    return (
      <FeatureBox>
        <Alert kind="info" mt={4}>
          You do not have permission to access Beams. Missing role permissions:{' '}
          <code>beams.list</code> and <code>beams.read</code>
        </Alert>
      </FeatureBox>
    );
  }

  const hasUnsupportedSortError = isError && isUnsupportedSortError(error);

  const isEmpty = isSuccess && data.items?.length === 0;

  return (
    <FeatureBox>
      <StyledFeatureHeaderTitle py={3} mb={2}>
        Beams
      </StyledFeatureHeaderTitle>

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
              // Reset to name:asc as this is the most likely to succeed
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

      {isSuccess && (
        <Flex justifyContent={'flex-end'} py={3} gap={5}>
          <Flex gap={3}>
            <b>Filter:</b>
            <Toggle isToggled={filterOwn} onToggle={handleToggleFilterOwn}>
              <Box pl={2}>Show my beams only</Box>
            </Toggle>
          </Flex>

          {!isEmpty && (
            <SortMenu
              items={[
                {
                  key: 'alias',
                  label: 'ID',
                  ascendingLabel: 'ID, A - Z',
                  descendingLabel: 'ID, Z - A',
                  ascendingOptionLabel: 'Alphabetical, A - Z',
                  descendingOptionLabel: 'Alphabetical, Z - A',
                  defaultOrder: 'ASC',
                },
                {
                  key: 'name',
                  label: 'UUID',
                  ascendingLabel: 'UUID, A - Z',
                  descendingLabel: 'UUID, Z - A',
                  ascendingOptionLabel: 'Alphabetical, A - Z',
                  descendingOptionLabel: 'Alphabetical, Z - A',
                  defaultOrder: 'ASC',
                },
                {
                  key: 'expires',
                  label: 'Expires',
                  ascendingLabel: 'Expires, Soonest',
                  descendingLabel: 'Expires, Latest',
                  ascendingOptionLabel: 'Soonest',
                  descendingOptionLabel: 'Latest',
                  defaultOrder: 'ASC',
                },
                {
                  key: 'user',
                  label: 'Owner',
                  ascendingLabel: 'Owner, A - Z',
                  descendingLabel: 'Owner, Z - A',
                  ascendingOptionLabel: 'Alphabetical, A - Z',
                  descendingOptionLabel: 'Alphabetical, Z - A',
                  defaultOrder: 'ASC',
                  hidden: filterOwn,
                },
              ]}
              onChange={(key, order) =>
                handleSortChanged({
                  fieldName: key,
                  dir: order,
                })
              }
              selectedKey={sortField}
              selectedOrder={sortDir === 'ASC' ? 'ASC' : 'DESC'}
            />
          )}
        </Flex>
      )}

      {isEmpty && <Empty />}

      {isSuccess && !isEmpty ? (
        <Table
          data={data?.items ?? []}
          serversideProps={{
            sort: sortType,
            setSort: handleSortChanged,
            serversideSearchPanel: <div style={{ backgroundColor: 'red' }} />,
          }}
          fetching={{
            fetchStatus: isPending || isFetching ? 'loading' : '',
            onFetchNext: hasNextPage ? handleFetchNext : undefined,
            onFetchPrev: hasPrevPage ? handleFetchPrev : undefined,
            disableLoadingIndicator: true,
          }}
          columns={[
            {
              key: 'alias',
              headerText: 'ID',
              render: ({ alias }) => (
                <Cell>
                  <Flex alignItems={'center'} gap={2}>
                    {valueOrPlaceholder(alias)}
                    {alias && (
                      <CopyButtonWrapper>
                        <CopyButton value={alias} />
                      </CopyButtonWrapper>
                    )}
                  </Flex>
                </Cell>
              ),
              isSortable: true,
            },
            {
              key: 'name',
              headerText: 'UUID',
              render: ({ name }) => (
                <Cell>
                  <Flex alignItems={'center'} gap={2}>
                    {valueOrPlaceholder(name)}
                    <CopyButtonWrapper>
                      <CopyButton value={name} />
                    </CopyButtonWrapper>
                  </Flex>
                </Cell>
              ),
              isSortable: true,
            },
            {
              key: 'expires',
              headerText: 'Expires',
              render: ({ expires }) => {
                if (!expires) return <Cell>-</Cell>;
                return (
                  <Cell>
                    <HoverTooltip
                      placement="top"
                      tipContent={format(parseISO(expires), 'PP, p z')}
                    >
                      <Text>{formatExpires(expires)}</Text>
                    </HoverTooltip>
                  </Cell>
                );
              },
              isSortable: true,
            },
            {
              key: 'user',
              headerText: 'Owner',
              render: ({ user }) => <Cell>{valueOrPlaceholder(user)}</Cell>,
              isSortable: true,
              isNonRender: filterOwn,
            },
          ]}
          emptyText="No Beams found"
        />
      ) : undefined}
    </FeatureBox>
  );
}

function formatExpires(expires: string) {
  const expiry = parseISO(expires);

  if (isBefore(expiry, new Date())) {
    return `Expired ${formatDistanceToNowStrict(expiry)} ago`;
  }

  try {
    return `${formatDistanceToNowStrict(expiry)} from now`;
  } catch {
    return expires;
  }
}

function valueOrPlaceholder(value: string | null | undefined) {
  return value || '-';
}

const isUnsupportedSortError = (error: Error) => {
  return !!error?.message && error.message.includes('unsupported sort');
};

const StyledFeatureHeaderTitle = styled(FeatureHeaderTitle)`
  flex-shrink: 0;
`;

const CopyButtonWrapper = styled(Box)`
  display: inline-flex;
  align-items: center;
  opacity: 0;

  tr:hover & {
    opacity: 1;
  }
`;

function Empty() {
  return (
    <>
      <Alert kind="info">No beams found</Alert>

      <EmptyWrapper>
        <H2 pb={3}>
          Create your first beam using the <code>tsh</code> CLI tool.
        </H2>
        <P>
          Check the{' '}
          <Link to={cfg.getBeamsQuickstartRoute()}>Beams Quickstart</Link> to
          get started.
        </P>
      </EmptyWrapper>
    </>
  );
}

const EmptyWrapper = styled.div`
  min-width: 480px;
  margin: ${({ theme }) => theme.space[6]}px auto 0 auto;
  padding: ${({ theme }) => theme.space[6]}px;
  border: 1px solid ${({ theme }) => theme.colors.interactive.tonal.neutral[2]};
  border-radius: ${({ theme }) => theme.radii[3]}px;
  background-color: ${({ theme }) => theme.colors.levels.surface};
  text-align: center;
`;
