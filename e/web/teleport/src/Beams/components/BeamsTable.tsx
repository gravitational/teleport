import {
  ArrowSquareOutIcon,
  CaretDownIcon,
  CaretUpDownIcon,
  CaretUpIcon,
  ComposedCheckbox,
  DataTable,
  Flex,
  PlayCircleIcon,
  Spinner,
  TerminalIcon,
  Text,
  Tooltip,
  type ColumnDef,
  type Row,
} from '@gravitational/design-system';
import {
  format,
  formatDistanceToNowStrict,
  isBefore,
  isValid,
  parseISO,
} from 'date-fns';
import {
  CSSProperties,
  createContext,
  useContext,
  useMemo,
  useState,
} from 'react';
import { Link } from 'react-router';
import styled, { useTheme } from 'styled-components';

import { ButtonIcon } from 'design';
import { CopyButton } from 'shared/components/CopyButton/CopyButton';

import { Beam, BeamsSortField } from 'e-teleport/services/beams/types';
import cfg from 'teleport/config';

import { BeamRowActions } from './BeamRowActions';
import {
  beamSortLabel,
  isBeamOwner,
  isProvisioning,
  publishedBeamUrl,
  SortDir,
} from './constants';
import { TcpAccessDialog } from './TcpAccessDialog';
import { BeamSelection } from './useBeamSelection';

export const HIGHLIGHT_MS = 3_000;

const HIGHLIGHT_ROW_STYLE: CSSProperties = {
  animation: `beamRowHighlight ${HIGHLIGHT_MS}ms ease-out forwards`,
};

type BeamsTableContextValue = {
  clusterId: string;
  clusterPublicUrl: string;
  currentUsername: string;
  canEdit: boolean;
  canRemove: boolean;
  canViewRecordings: boolean;
  recordingHostnames: ReadonlySet<string>;
  beams: Beam[];
  selection: BeamSelection;
  sortField: BeamsSortField;
  sortDir: SortDir;
  onSort: (field: BeamsSortField, dir: SortDir) => void;
  onRequestDelete: (beams: Beam[]) => void;
  openTcpDialog: (beam: Beam) => void;
};

const BeamsTableContext = createContext<BeamsTableContextValue | null>(null);

function useBeamsTableContext(): BeamsTableContextValue {
  return useContext(BeamsTableContext)!;
}

type BeamsTableProps = {
  beams: Beam[];
  clusterId: string;
  clusterPublicUrl: string;
  currentUsername: string;
  canEdit: boolean;
  canRemove: boolean;
  canViewRecordings?: boolean;
  recordingHostnames?: ReadonlySet<string>;
  selection: BeamSelection;
  sortField: BeamsSortField;
  sortDir: SortDir;
  highlightedName: string | null;
  onSort: (field: BeamsSortField, dir: SortDir) => void;
  onRequestDelete: (beams: Beam[]) => void;
};

const NO_RECORDINGS: ReadonlySet<string> = new Set();

export function BeamsTable({
  beams,
  clusterId,
  clusterPublicUrl,
  currentUsername,
  canEdit,
  canRemove,
  canViewRecordings = false,
  recordingHostnames = NO_RECORDINGS,
  selection,
  sortField,
  sortDir,
  highlightedName,
  onSort,
  onRequestDelete,
}: BeamsTableProps) {
  const theme = useTheme();
  const [tcpDialogBeam, setTcpDialogBeam] = useState<Beam | null>(null);

  const selectedRowStyle = useMemo<CSSProperties>(
    () => ({ backgroundColor: theme.colors.interactive.tonal.primary[2] }),
    [theme]
  );

  const contextValue: BeamsTableContextValue = {
    clusterId,
    clusterPublicUrl,
    currentUsername,
    canEdit,
    canRemove,
    canViewRecordings,
    recordingHostnames,
    beams,
    selection,
    sortField,
    sortDir,
    onSort,
    onRequestDelete,
    openTcpDialog: setTcpDialogBeam,
  };

  const columns = canRemove ? COLUMNS_WITH_SELECT : COLUMNS_WITHOUT_SELECT;

  return (
    <BeamsTableContext.Provider value={contextValue}>
      <TableWrapper $hasSelectColumn={canRemove}>
        <DataTable<Beam>
          data={beams}
          columns={columns}
          emptyText="No beams found"
          autoResetPageIndex={false}
          getRowId={beam => beam.name}
          row={{
            getStyle: beam => {
              if (beam.name === highlightedName) return HIGHLIGHT_ROW_STYLE;
              if (selection.has(beam.name)) return selectedRowStyle;
              return {};
            },
          }}
        />
        {tcpDialogBeam && (
          <TcpAccessDialog
            appName={tcpDialogBeam.app_name}
            vnetUrl={publishedBeamUrl(tcpDialogBeam, clusterPublicUrl)}
            onClose={() => setTcpDialogBeam(null)}
          />
        )}
      </TableWrapper>
    </BeamsTableContext.Provider>
  );
}

function SelectAllHeader() {
  const { beams, selection } = useBeamsTableContext();
  const allSelected =
    beams.length > 0 && beams.every(b => selection.has(b.name));
  const someSelected = beams.some(b => selection.has(b.name)) && !allSelected;

  const ariaLabel = allSelected
    ? 'Clear selected beams on this page'
    : 'Select all beams on this page';
  return (
    <SelectionCheckbox
      ariaLabel={ariaLabel}
      checked={allSelected ? true : someSelected ? 'indeterminate' : false}
      onCheckedChange={() => selection.toggleAll(beams)}
    />
  );
}

function SortableHeader({ field }: { field: BeamsSortField }) {
  const { sortField, sortDir, onSort } = useBeamsTableContext();
  const label = beamSortLabel(field);
  const active = sortField === field;
  const Icon = !active
    ? CaretUpDownIcon
    : sortDir === 'ASC'
      ? CaretUpIcon
      : CaretDownIcon;
  const nextDir = active && sortDir === 'ASC' ? 'DESC' : 'ASC';

  return (
    <HeaderSortButton
      type="button"
      aria-label={`Sort by ${label}`}
      onClick={() => onSort(field, nextDir)}
    >
      {label}
      <Icon boxSize={4} ml={1} />
    </HeaderSortButton>
  );
}

function RowSelectCell({ row }: { row: Row<Beam> }) {
  const { selection } = useBeamsTableContext();
  const beam = row.original;
  return (
    <SelectionCheckbox
      ariaLabel={`Select beam ${beam.alias || beam.name}`}
      checked={selection.has(beam.name)}
      onCheckedChange={details =>
        selection.toggleOne(beam, details.checked === true)
      }
    />
  );
}

function AliasCell({ row }: { row: Row<Beam> }) {
  const beam = row.original;
  const alias = beam.alias || beam.name;
  const provisioning = isProvisioning(beam);
  return (
    <Flex alignItems="center" gap={2}>
      {provisioning && (
        <Tooltip content="This beam is still provisioning">
          <Spinner size="xs" color="text.muted" />
        </Tooltip>
      )}
      <BeamAlias>{alias}</BeamAlias>
      <CopyButtonWrapper>
        <CopyButton value={alias} />
      </CopyButtonWrapper>
    </Flex>
  );
}

function PublishedCell({ row }: { row: Row<Beam> }) {
  const { clusterPublicUrl, currentUsername, openTcpDialog } =
    useBeamsTableContext();
  const beam = row.original;
  const provisioning = isProvisioning(beam);
  const published = !!beam.publish && !provisioning;
  if (!published) return null;

  const alias = beam.alias || beam.name;
  const url = publishedBeamUrl(beam, clusterPublicUrl);
  const isTcp = url.startsWith('tcp://');
  const canOpen = isBeamOwner(beam, currentUsername);

  return (
    <Flex alignItems="center" gap={2}>
      <PublishedDot />
      <Text textStyle="sm">Published</Text>
      {isTcp ? (
        canOpen ? (
          <ProtocolBadge
            onClick={() => openTcpDialog(beam)}
            aria-label={`Show TCP access instructions for ${alias}`}
          >
            TCP
            <TerminalIcon boxSize={4} ml={2} />
          </ProtocolBadge>
        ) : (
          <Tooltip content="You don't have permission to connect to this app">
            <ProtocolBadge as="span" data-muted>
              TCP
              <TerminalIcon boxSize={4} ml={2} />
            </ProtocolBadge>
          </Tooltip>
        )
      ) : canOpen ? (
        <ProtocolBadge
          as="a"
          href={url}
          target="_blank"
          rel="noreferrer"
          aria-label={`Open published app for ${alias}`}
        >
          HTTPS
          <ArrowSquareOutIcon boxSize={4} ml={2} />
        </ProtocolBadge>
      ) : (
        <Tooltip content="You don't have permission to open this app">
          <ProtocolBadge as="span" data-muted>
            HTTPS
            <ArrowSquareOutIcon boxSize={4} ml={2} />
          </ProtocolBadge>
        </Tooltip>
      )}
    </Flex>
  );
}

function UserCell({ row }: { row: Row<Beam> }) {
  return <MutedRowText>{row.original.user || '-'}</MutedRowText>;
}

function ExpirationCell({ row }: { row: Row<Beam> }) {
  return <ExpirationLabel expires={row.original.expires} />;
}

function ActionsCell({ row }: { row: Row<Beam> }) {
  const {
    clusterId,
    canEdit,
    canRemove,
    canViewRecordings,
    recordingHostnames,
    onRequestDelete,
  } = useBeamsTableContext();
  const beam = row.original;
  const hasRecording =
    canViewRecordings && recordingHostnames.has(`beam-${beam.name}`);

  return (
    <Flex alignItems="center" justifyContent="flex-end" gap={3}>
      {hasRecording && (
        <SessionRecordingLink clusterId={clusterId} beam={beam} />
      )}
      <BeamRowActions
        beam={beam}
        clusterId={clusterId}
        canEdit={canEdit}
        canRemove={canRemove}
        onRequestDelete={b => onRequestDelete([b])}
      />
    </Flex>
  );
}

// A beam's SSH sessions are recorded under the hostname `beam-<beam.name>`,
// which is the resource name the recordings list filters on.
function SessionRecordingLink({
  clusterId,
  beam,
}: {
  clusterId: string;
  beam: Beam;
}) {
  const resource = `beam-${beam.name}`;
  const to = new Date();
  const from = new Date(to.getTime() - 24 * 60 * 60 * 1000);
  const params = new URLSearchParams({
    resources: resource,
    from: from.toISOString(),
    to: to.toISOString(),
  });
  const url = `${cfg.getRecordingsRoute(clusterId)}?${params.toString()}`;

  const label = beam.alias || beam.name;
  return (
    <Tooltip content="Session recordings">
      <ButtonIcon
        as={Link}
        to={url}
        size={1}
        color="text.slightlyMuted"
        aria-label={`View session recordings for ${label}`}
      >
        <PlayCircleIcon boxSize={6} />
      </ButtonIcon>
    </Tooltip>
  );
}

const SELECT_COLUMN: ColumnDef<Beam> = {
  id: 'select',
  header: SelectAllHeader,
  cell: RowSelectCell,
};

const ALIAS_COLUMN: ColumnDef<Beam> = {
  id: 'alias',
  header: () => <SortableHeader field="alias" />,
  cell: AliasCell,
};

const PUBLISHED_COLUMN: ColumnDef<Beam> = {
  id: 'published',
  header: () => null,
  cell: PublishedCell,
};

const USER_COLUMN: ColumnDef<Beam> = {
  id: 'user',
  header: () => <SortableHeader field="user" />,
  cell: UserCell,
};

const EXPIRES_COLUMN: ColumnDef<Beam> = {
  id: 'expires',
  header: () => <SortableHeader field="expires" />,
  cell: ExpirationCell,
};

const ACTIONS_COLUMN: ColumnDef<Beam> = {
  id: 'actions',
  header: () => null,
  cell: ActionsCell,
};

const COLUMNS_WITH_SELECT: ColumnDef<Beam>[] = [
  SELECT_COLUMN,
  ALIAS_COLUMN,
  PUBLISHED_COLUMN,
  USER_COLUMN,
  EXPIRES_COLUMN,
  ACTIONS_COLUMN,
];

const COLUMNS_WITHOUT_SELECT: ColumnDef<Beam>[] = [
  ALIAS_COLUMN,
  PUBLISHED_COLUMN,
  USER_COLUMN,
  EXPIRES_COLUMN,
  ACTIONS_COLUMN,
];

function SelectionCheckbox({
  ariaLabel,
  checked,
  onCheckedChange,
}: {
  ariaLabel: string;
  checked: boolean | 'indeterminate';
  onCheckedChange: (details: { checked: boolean | 'indeterminate' }) => void;
}) {
  return (
    <ComposedCheckbox
      size="sm"
      aria-label={ariaLabel}
      checked={checked}
      onCheckedChange={onCheckedChange}
    />
  );
}

function ExpirationLabel({ expires }: { expires: string }) {
  if (!expires) return <>-</>;
  const expiry = parseISO(expires);
  if (!isValid(expiry)) return <>-</>;
  return (
    <Tooltip placement="top" content={format(expiry, 'PP, p z')}>
      <MutedRowText>{formatExpires(expiry)}</MutedRowText>
    </Tooltip>
  );
}

function formatExpires(expiry: Date) {
  const distance = formatDistanceToNowStrict(expiry);
  return isBefore(expiry, new Date())
    ? `Expired ${distance} ago`
    : `Expires in ${distance}`;
}

const TableWrapper = styled.div<{ $hasSelectColumn: boolean }>`
  background: ${({ theme }) => theme.colors.levels.surface};
  border: 1px solid ${({ theme }) => theme.colors.interactive.tonal.neutral[2]};
  border-radius: ${({ theme }) => theme.radii[2]}px;

  table {
    thead tr {
      background-color: ${({ theme }) =>
        theme.type === 'light'
          ? theme.colors.levels.deep
          : theme.colors.levels.elevated};
    }

    ${({ theme, $hasSelectColumn }) =>
      $hasSelectColumn &&
      `
      thead > tr > th:first-child,
      tbody > tr > td:first-child {
        width: 1%;
        white-space: nowrap;
        padding-left: ${theme.space[4]}px;
        padding-right: ${theme.space[2]}px;
      }
    `}

    ${({ $hasSelectColumn }) => {
      const aliasIndex = $hasSelectColumn ? 2 : 1;
      return `
        thead > tr > th:nth-child(${aliasIndex}),
        tbody > tr > td:nth-child(${aliasIndex}) {
          width: 1%;
          white-space: nowrap;
        }
      `;
    }}

    thead > tr > th:last-child,
    tbody > tr > td:last-child {
      padding-right: ${({ theme }) => theme.space[4]}px;
    }

    tbody tr {
      height: 68px;
      &:hover {
        background-color: ${({ theme }) =>
          theme.colors.interactive.tonal.neutral[0]};
        &:after {
          box-shadow: none;
        }
      }
    }
  }
`;

const MutedRowText = styled(Text)`
  color: ${({ theme }) => theme.colors.text.slightlyMuted};
`;

const CopyButtonWrapper = styled.span`
  display: inline-flex;
  align-items: center;
  opacity: 0;
  transition: opacity 150ms;

  tr:hover &,
  &:focus-within {
    opacity: 1;
  }
`;

const BeamAlias = styled.span`
  font-weight: ${({ theme }) => theme.fontWeights.bold};
  font-size: ${({ theme }) => theme.fontSizes[2]}px;
`;

const PublishedDot = styled.span`
  display: inline-block;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: ${({ theme }) => theme.colors.interactive.solid.success.default};
`;

const ProtocolBadge = styled.button`
  display: inline-flex;
  align-items: center;
  height: 28px;
  padding: 0 ${({ theme }) => theme.space[2]}px;
  border-radius: ${({ theme }) => theme.radii[2]}px;
  border: 1px solid ${({ theme }) => theme.colors.interactive.tonal.neutral[2]};
  background: transparent;
  color: ${({ theme }) => theme.colors.text.main};
  font-family: ${({ theme }) => theme.fonts.mono};
  font-size: ${({ theme }) => theme.fontSizes[0]}px;
  cursor: pointer;
  text-transform: uppercase;
  gap: ${({ theme }) => theme.space[1]}px;

  &:hover {
    background: ${({ theme }) => theme.colors.interactive.tonal.neutral[0]};
  }

  &:focus {
    outline: none;
  }

  &:focus-visible {
    outline: 2px solid ${({ theme }) => theme.colors.brand};
    outline-offset: 2px;
  }

  &[data-muted] {
    color: ${({ theme }) => theme.colors.text.disabled};
    cursor: not-allowed;
  }
`;

const HeaderSortButton = styled.button`
  display: inline-flex;
  align-items: center;
  padding: 0;
  border: none;
  background: transparent;
  color: ${({ theme }) => theme.colors.text.main};
  cursor: pointer;
  font: inherit;
  font-weight: ${({ theme }) => theme.fontWeights.bold};
  border-radius: ${({ theme }) => theme.radii[1]}px;

  &:focus {
    outline: none;
  }

  &:focus-visible {
    outline: 2px solid ${({ theme }) => theme.colors.brand};
    outline-offset: 2px;
  }
`;
