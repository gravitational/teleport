import {
  Box,
  Flex,
  H2,
  InfoIcon,
  P1,
  Spinner,
} from '@gravitational/design-system';
import { useEffect, useState } from 'react';
import { Link } from 'react-router';
import styled, { createGlobalStyle } from 'styled-components';

import { Alert } from 'design/Alert/Alert';

import cfg from 'e-teleport/config';
import { Beam, BeamsSortField } from 'e-teleport/services/beams/types';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout/Layout';
import useStickyClusterId from 'teleport/useStickyClusterId';
import useTeleport from 'teleport/useTeleport';

import { CreateBeamButton } from './components';
import { BeamsTable, HIGHLIGHT_MS } from './components/BeamsTable';
import { BeamsToolbar } from './components/BeamsToolbar';
import { ConfirmDeleteDialog } from './components/ConfirmDeleteDialog';
import { PAGE_SIZE, SortDir } from './components/constants';
import { useDeleteBeams, useListBeams } from './components/useBeamMutations';
import { useBeamRecordingHostnames } from './components/useBeamRecordingHostnames';
import { useBeamSelection } from './components/useBeamSelection';
import { useBeamsListParams } from './components/useBeamsListParams';

const EMPTY_HOSTNAMES: ReadonlySet<string> = new Set();

export function BeamsList() {
  const ctx = useTeleport();
  const { clusterId } = useStickyClusterId();
  const flags = ctx.getFeatureFlags();
  const access = ctx.storeUser.getBeamAccess();
  const canList = flags.listBeam && flags.readBeam;
  const canViewRecordings = flags.recordings;

  const params = useBeamsListParams();
  const users = params.filterOwn ? [ctx.storeUser.getUsername()] : undefined;

  const selection = useBeamSelection();
  const [pendingDelete, setPendingDelete] = useState<Beam[] | null>(null);
  const [highlightedName, setHighlightedName] = useState<string | null>(null);

  useEffect(() => {
    if (!highlightedName) return;
    const id = setTimeout(() => setHighlightedName(null), HIGHLIGHT_MS);
    return () => clearTimeout(id);
  }, [highlightedName]);

  const handleSort = (field: BeamsSortField, dir: SortDir) => {
    selection.clear();
    params.setSort(field, dir);
  };

  const handleToggleFilterOwn = () => {
    selection.clear();
    params.toggleFilterOwn();
  };

  const remove = useDeleteBeams(clusterId, {
    onSuccess: deleted => {
      selection.removeMany(deleted);
      setPendingDelete(null);
    },
  });

  const { isPending, isFetching, isSuccess, isError, error, data } =
    useListBeams({
      clusterId,
      pageSize: PAGE_SIZE,
      pageToken: params.pageToken,
      sortField: params.sortField,
      sortDir: params.sortDir,
      users,
      enabled: canList,
    });

  const recordings = useBeamRecordingHostnames({
    clusterId,
    enabled: canList && canViewRecordings,
  });
  const recordingHostnames = recordings.data ?? EMPTY_HOSTNAMES;

  const onBeamCreated = (beam: Beam) => {
    handleSort('expires', 'DESC');
    setHighlightedName(beam.name);
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
  const pageBeams = data?.items ?? [];
  const noResults = isSuccess && pageBeams.length === 0;
  const nextPageToken = data?.next_page_token ?? '';

  return (
    <FeatureBox pl={5} pr={5}>
      <HighlightKeyframes />
      <FeatureHeader
        justifyContent="space-between"
        alignItems="center"
        mb={3}
        mt={2}
      >
        <FeatureHeaderTitle py={3}>Beams</FeatureHeaderTitle>
        <Flex alignItems="center" gap={3}>
          <QuickstartPill to={cfg.getBeamsQuickstartRoute()}>
            <InfoIcon boxSize={4} mr={2} />
            For a refresher, visit the{' '}
            <QuickstartLink>Beams Quickstart</QuickstartLink>
          </QuickstartPill>
          {access.create && (
            <CreateBeamButton clusterId={clusterId} onCreated={onBeamCreated} />
          )}
        </Flex>
      </FeatureHeader>

      {isPending && (
        <Box data-testid="loading" textAlign="center" m={10}>
          <Spinner size="lg" />
        </Box>
      )}

      {hasUnsupportedSortError && (
        <Alert
          kind="warning"
          primaryAction={{
            content: 'Reset sort',
            onClick: () => params.setSort('name', 'ASC'),
          }}
        >
          {error.message}
        </Alert>
      )}

      {isError && !hasUnsupportedSortError && (
        <Alert kind="danger">{error.message}</Alert>
      )}

      {isSuccess && (
        <BeamsToolbar
          paging={{
            pageIndex: params.pageIndex,
            pageItemCount: pageBeams.length,
            hasPrevPage: params.hasPrevPage,
            hasNextPage: !!nextPageToken,
            isFetching,
            onGoPrev: params.goPrev,
            onGoNext: () => nextPageToken && params.goNext(nextPageToken),
          }}
          sort={{
            field: params.sortField,
            dir: params.sortDir,
            onChange: handleSort,
          }}
          filter={{
            filterOwn: params.filterOwn,
            onToggle: handleToggleFilterOwn,
          }}
          bulkDelete={{
            enabled: access.remove,
            isPending: remove.isPending,
            onRequest: setPendingDelete,
          }}
          selection={selection}
          noResults={noResults}
        />
      )}

      {noResults && <Empty />}

      {isSuccess && !noResults && (
        <BeamsTable
          beams={pageBeams}
          clusterId={clusterId}
          clusterPublicUrl={ctx.storeUser.getClusterPublicUrl()}
          currentUsername={ctx.storeUser.getUsername()}
          canEdit={access.edit}
          canRemove={access.remove}
          canViewRecordings={canViewRecordings}
          recordingHostnames={recordingHostnames}
          selection={selection}
          sortField={params.sortField}
          sortDir={params.sortDir}
          highlightedName={highlightedName}
          onSort={handleSort}
          onRequestDelete={setPendingDelete}
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

function Empty() {
  return (
    <EmptyWrapper>
      <H2 mb={2}>No beams found</H2>
      <P1>
        Create your first beam with the <strong>Create beam</strong> button
        above, or use the <code>tsh</code> CLI. See the{' '}
        <Link to={cfg.getBeamsQuickstartRoute()}>Beams Quickstart</Link> to get
        started.
      </P1>
    </EmptyWrapper>
  );
}

const isUnsupportedSortError = (error: Error) =>
  !!error?.message && error.message.includes('unsupported sort');

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

const QuickstartPill = styled(Link)`
  display: inline-flex;
  align-items: center;
  height: 32px;
  padding: 0 ${({ theme }) => theme.space[3]}px;
  border-radius: ${({ theme }) => theme.radii[2]}px;
  border: none;
  background: ${({ theme }) => theme.colors.levels.surface};
  color: ${({ theme }) => theme.colors.text.main};
  text-decoration: none;
  font-size: ${({ theme }) => theme.fontSizes[1]}px;

  &:hover {
    background: ${({ theme }) => theme.colors.levels.elevated};
  }

  &:focus {
    outline: none;
  }

  &:focus-visible {
    outline: 2px solid ${({ theme }) => theme.colors.brand};
    outline-offset: 2px;
  }
`;

const QuickstartLink = styled.span`
  color: ${({ theme }) => theme.colors.interactive.solid.primary.default};
  font-weight: ${({ theme }) => theme.fontWeights.bold};
  margin-left: ${({ theme }) => theme.space[1]}px;
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
