import { keepPreviousData, useQuery } from '@tanstack/react-query';
import {
  format,
  formatDistanceToNowStrict,
  isBefore,
  parseISO,
} from 'date-fns';
import { useEffect, useState } from 'react';
import { Link } from 'react-router';
import styled, { createGlobalStyle } from 'styled-components';

import { ButtonWarning } from 'design';
import { Alert } from 'design/Alert/Alert';
import Box from 'design/Box/Box';
import { CheckboxInput } from 'design/Checkbox/Checkbox';
import { Cell } from 'design/DataTable';
import Table from 'design/DataTable/Table';
import { SortType } from 'design/DataTable/types';
import Flex from 'design/Flex';
import { NewTab } from 'design/Icon';
import { Indicator } from 'design/Indicator/Indicator';
import Text, { H2 } from 'design/Text';
import { P } from 'design/Text/Text';
import { Toggle } from 'design/Toggle';
import { HoverTooltip } from 'design/Tooltip/HoverTooltip';
import { SortMenu } from 'shared/components/Controls/SortMenu';
import { CopyButton } from 'shared/components/CopyButton/CopyButton';

import cfg from 'e-teleport/config';
import { beamsService } from 'e-teleport/services/beams/beams';
import { Beam, BeamsSortField } from 'e-teleport/services/beams/types';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout/Layout';
import useStickyClusterId from 'teleport/useStickyClusterId';
import useTeleport from 'teleport/useTeleport';

import { BeamRowActions, CreateBeamButton } from './components';
import { ConfirmDeleteDialog } from './components/ConfirmDeleteDialog';
import {
  isBeamOwner,
  isProvisioning,
  publishedBeamUrl,
} from './components/constants';
import { useDeleteBeams } from './components/useBeamMutations';
import { useBeamsListParams } from './components/useBeamsListParams';

const PAGE_SIZE = 20;
const HIGHLIGHT_MS = 3_000;

export function BeamsList() {
  const ctx = useTeleport();
  const { clusterId } = useStickyClusterId();
  const flags = ctx.getFeatureFlags();
  const access = ctx.storeUser.getBeamAccess();
  const canList = flags.listBeam && flags.readBeam;

  const {
    pageToken,
    sortField,
    sortDir,
    filterOwn,
    hasPrevPage,
    goNext,
    goPrev,
    setSort,
    toggleFilterOwn,
  } = useBeamsListParams();
  const users = filterOwn ? [ctx.storeUser.getUsername()] : undefined;

  const [highlightedName, setHighlightedName] = useState<string | null>(null);
  useEffect(() => {
    if (!highlightedName) return;
    const id = setTimeout(() => setHighlightedName(null), HIGHLIGHT_MS);
    return () => clearTimeout(id);
  }, [highlightedName]);

  const [selection, setSelection] = useState<Map<string, Beam>>(
    () => new Map()
  );

  // pending and selection both states are needed to support individual deletes
  // when multiple beams are selected.
  const [pendingDelete, setPendingDelete] = useState<Beam[] | null>(null);

  const remove = useDeleteBeams(clusterId, {
    onSuccess: deleted => {
      setSelection(prev => {
        const next = new Map(prev);
        deleted.forEach(b => next.delete(b.name));
        return next;
      });
      setPendingDelete(null);
    },
  });

  const { isPending, isFetching, isSuccess, isError, error, data } = useQuery({
    enabled: canList,
    queryKey: ['beams', clusterId, pageToken, sortField, sortDir, users],
    queryFn: ({ signal }) =>
      beamsService.listBeams(
        {
          pageSize: PAGE_SIZE,
          pageToken,
          sortField: sortField as BeamsSortField,
          sortDir: sortDir === 'DESC' ? 'desc' : 'asc',
          users,
        },
        clusterId,
        signal
      ),
    placeholderData: keepPreviousData,
    staleTime: 30_000,
  });

  const onBeamCreated = (beam: Beam) => {
    setSort('expires', 'DESC');
    setHighlightedName(beam.name);
  };

  const toggleSelectOne = (beam: Beam, checked: boolean) => {
    setSelection(prev => {
      const next = new Map(prev);
      if (checked) next.set(beam.name, beam);
      else next.delete(beam.name);
      return next;
    });
  };

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
  const isEmpty = isSuccess && !data.items?.length;
  const sort: SortType = { fieldName: sortField, dir: sortDir };

  return (
    <FeatureBox>
      <HighlightKeyframes />
      <FeatureHeader justifyContent="space-between" alignItems="center">
        <FeatureHeaderTitle py={3}>My Beams</FeatureHeaderTitle>
        {access.create && (
          <CreateBeamButton clusterId={clusterId} onCreated={onBeamCreated} />
        )}
      </FeatureHeader>

      {isPending && (
        <Box data-testid="loading" textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}

      {hasUnsupportedSortError && (
        <Alert
          kind="warning"
          primaryAction={{
            content: 'Reset sort',
            onClick: () => setSort('name', 'ASC'),
          }}
        >
          {error.message}
        </Alert>
      )}

      {isError && !hasUnsupportedSortError && (
        <Alert kind="danger">{error.message}</Alert>
      )}

      {isSuccess && (
        <Flex justifyContent="flex-end" alignItems="center" py={3} gap={3}>
          {access.remove && selection.size > 0 && (
            <DeleteButton
              disabled={remove.isPending}
              onClick={() => setPendingDelete(Array.from(selection.values()))}
            >
              Delete ({selection.size})
            </DeleteButton>
          )}

          <Toggle isToggled={filterOwn} onToggle={toggleFilterOwn}>
            <Box pl={2}>Show my beams only</Box>
          </Toggle>

          {!isEmpty && (
            <SortMenu
              selectedKey={sortField}
              selectedOrder={sortDir}
              onChange={(key, order) => setSort(key, order)}
              items={SORT_ITEMS.map(item =>
                item.key === 'user' ? { ...item, hidden: filterOwn } : item
              )}
            />
          )}
        </Flex>
      )}

      {isEmpty && <Empty />}

      {isSuccess && !isEmpty && (
        <Table<Beam>
          data={data.items ?? []}
          pagination={{ pagerPosition: 'top' }}
          row={{
            getKey: beam => beam.name,
            getStyle: beam =>
              beam.name === highlightedName ? HIGHLIGHT_ROW_STYLE : {},
          }}
          serversideProps={{
            sort,
            setSort: next => setSort(next.fieldName, next.dir),
            serversideSearchPanel: <Box />,
          }}
          fetching={{
            fetchStatus: isPending || isFetching ? 'loading' : '',
            onFetchNext: data.next_page_token
              ? () => goNext(data.next_page_token)
              : undefined,
            onFetchPrev: hasPrevPage ? goPrev : undefined,
            disableLoadingIndicator: true,
          }}
          columns={[
            {
              altKey: 'select',
              render: beam => (
                <Cell style={{ width: 32 }}>
                  <CheckboxInput
                    aria-label={`Select beam ${beam.alias || beam.name}`}
                    checked={selection.has(beam.name)}
                    onChange={e => toggleSelectOne(beam, e.target.checked)}
                  />
                </Cell>
              ),
              isNonRender: !access.remove,
            },
            {
              key: 'alias',
              headerText: 'ID',
              render: beam => (
                <CopyableCell
                  value={beam.alias}
                  isProvisioning={isProvisioning(beam)}
                />
              ),
              isSortable: true,
            },
            {
              altKey: 'published_url',
              headerText: 'Published URL',
              render: beam => (
                <PublishedUrlCell
                  url={
                    isProvisioning(beam)
                      ? ''
                      : publishedBeamUrl(
                          beam,
                          ctx.storeUser.getClusterPublicUrl()
                        )
                  }
                  appName={beam.app_name}
                  canOpen={isBeamOwner(beam, ctx.storeUser.getUsername())}
                />
              ),
            },
            {
              key: 'expires',
              headerText: 'Expires',
              render: ({ expires }) => {
                if (!expires) return <Cell>-</Cell>;
                const expiry = parseISO(expires);
                return (
                  <Cell>
                    <HoverTooltip
                      placement="top"
                      tipContent={format(expiry, 'PP, p z')}
                    >
                      <Text>{formatExpires(expiry)}</Text>
                    </HoverTooltip>
                  </Cell>
                );
              },
              isSortable: true,
            },
            {
              key: 'user',
              headerText: 'Owner',
              render: ({ user }) => <Cell>{user || '-'}</Cell>,
              isSortable: true,
              isNonRender: filterOwn,
            },
            {
              altKey: 'actions',
              render: beam => (
                <Cell>
                  <BeamRowActions
                    beam={beam}
                    clusterId={clusterId}
                    canEdit={access.edit}
                    canRemove={access.remove}
                    onRequestDelete={b => setPendingDelete([b])}
                  />
                </Cell>
              ),
            },
          ]}
          emptyText="No beams found"
        />
      )}

      {pendingDelete && (
        <ConfirmDeleteDialog
          beams={pendingDelete}
          isPending={remove.isPending}
          onClose={() => setPendingDelete(null)}
          onConfirm={() => remove.mutate(pendingDelete)}
        />
      )}
    </FeatureBox>
  );
}

function PublishedUrlCell({
  url,
  appName,
  canOpen,
}: {
  url: string;
  appName: string;
  canOpen: boolean;
}) {
  if (!url) {
    return (
      <Cell>
        <Text color="text.muted">—</Text>
      </Cell>
    );
  }
  if (!canOpen) {
    return (
      <Cell>
        <HoverTooltip tipContent="You don't have permission to open this app">
          <UrlText muted>{url}</UrlText>
        </HoverTooltip>
      </Cell>
    );
  }
  const isTcp = url.startsWith('tcp://');
  return (
    <Cell>
      <Flex alignItems="center" gap={2}>
        {isTcp ? (
          <HoverTooltip
            tipContent={`Run "tsh proxy app ${appName}" to start a local proxy, then connect your client to the proxy address (e.g. "localhost:54321").`}
          >
            <UrlText>{url}</UrlText>
          </HoverTooltip>
        ) : (
          <UrlLink href={url} target="_blank" rel="noreferrer">
            {url}
            <NewTab size="small" ml={1} />
          </UrlLink>
        )}
        <CopyButtonWrapper>
          <CopyButton value={url} />
        </CopyButtonWrapper>
      </Flex>
    </Cell>
  );
}

function CopyableCell({
  value,
  isProvisioning,
}: {
  value: string;
  isProvisioning?: boolean;
}) {
  return (
    <Cell>
      <Flex alignItems="center" gap={2}>
        {isProvisioning && (
          <HoverTooltip tipContent="This beam is still provisioning">
            <Indicator size={12} delay="none" color="text.muted" />
          </HoverTooltip>
        )}
        {value || '-'}
        {value && (
          <CopyButtonWrapper>
            <CopyButton value={value} />
          </CopyButtonWrapper>
        )}
      </Flex>
    </Cell>
  );
}

function Empty() {
  return (
    <EmptyWrapper>
      <H2 mb={2}>No beams found</H2>
      <P>
        Create your first beam with the <strong>Create beam</strong> button
        above, or use the <code>tsh</code> CLI. See the{' '}
        <Link to={cfg.getBeamsQuickstartRoute()}>Beams Quickstart</Link> to get
        started.
      </P>
    </EmptyWrapper>
  );
}

function formatExpires(expiry: Date) {
  const distance = formatDistanceToNowStrict(expiry);
  return isBefore(expiry, new Date())
    ? `Expired ${distance} ago`
    : `${distance} from now`;
}

const isUnsupportedSortError = (error: Error) => {
  return !!error?.message && error.message.includes('unsupported sort');
};

const SORT_ITEMS = [
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
  },
] as const;

const CopyButtonWrapper = styled(Box)`
  display: inline-flex;
  align-items: center;
  opacity: 0;

  tr:hover & {
    opacity: 1;
  }
`;

const DeleteButton = styled(ButtonWarning)`
  &:disabled {
    background-color: ${({ theme }) =>
      theme.colors.interactive.solid.danger.default};
    color: ${({ theme }) => theme.colors.text.primaryInverse};
    opacity: 0.5;
  }
`;

const UrlLink = styled.a`
  display: inline-flex;
  align-items: center;
  color: ${({ theme }) => theme.colors.interactive.solid.accent.default};
  font-family: ${({ theme }) => theme.fonts.mono};
  font-size: ${({ theme }) => theme.fontSizes[1]}px;
  text-decoration: none;
  word-break: break-all;

  &:hover,
  &:focus-visible {
    text-decoration: underline;
  }
`;

const HIGHLIGHT_ROW_STYLE = {
  animation: `beamRowHighlight ${HIGHLIGHT_MS}ms ease-out forwards`,
};

const HighlightKeyframes = createGlobalStyle`
  @keyframes beamRowHighlight {
    0% {
      background-color: ${({ theme }) => theme.colors.interactive.tonal.success[2]};
    }
    100% {
      background-color: transparent;
    }
  }
`;

const UrlText = styled.span<{ muted?: boolean }>`
  font-family: ${({ theme }) => theme.fonts.mono};
  font-size: ${({ theme }) => theme.fontSizes[1]}px;
  word-break: break-all;
  ${({ muted, theme }) =>
    muted && `color: ${theme.colors.text.disabled}; cursor: not-allowed;`}
`;

const EmptyWrapper = styled.div`
  max-width: 540px;
  margin: ${({ theme }) => theme.space[6]}px auto 0 auto;
  padding: ${({ theme }) => theme.space[6]}px;
  border: 1px solid ${({ theme }) => theme.colors.interactive.tonal.neutral[2]};
  border-radius: ${({ theme }) => theme.radii[3]}px;
  background-color: ${({ theme }) => theme.colors.levels.surface};
  text-align: center;
`;
