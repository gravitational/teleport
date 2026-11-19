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

import {
  ComposedCheckbox,
  DataTable,
  type ColumnDef,
  type Row,
} from '@gravitational/design-system';
import { useQueryClient } from '@tanstack/react-query';
import {
  createContext,
  useContext,
  useMemo,
  useState,
  type MouseEventHandler,
} from 'react';
import styled, { useTheme } from 'styled-components';

import { Box, Flex, Indicator, Text } from 'design';
import { Danger } from 'design/Alert';
import { ButtonBorder, ButtonPrimaryBorder } from 'design/Button';
import InputSearch from 'design/DataTable/InputSearch';
import { Check, CircleCross, Warning } from 'design/Icon';
import { MenuButton, MenuItem } from 'shared/components/MenuAction';
import { useToastNotifications } from 'shared/components/ToastNotification';
import { pluralize } from 'shared/utils/text';

import { SlidingSidePanel } from 'teleport/components/SlidingSidePanel';
import {
  integrationService,
  UserTask,
  type IntegrationWithSummary,
} from 'teleport/services/integrations';

import { formatRelativeDate } from './IaCIntegrationOverview';
import { MarkAsResolvedDialog } from './MarkAsResolvedDialog';
import { UserTaskDrawer } from './UserTaskDrawer';

type EventTypeFilter = 'All' | TaskEventType;

const taskResourceTypeFilters = ['All', 'EC2', 'RDS', 'EKS'] as const;
export type TaskResourceTypeFilter = (typeof taskResourceTypeFilters)[number];

enum DateOrder {
  NewestFirst = 'Newest first',
  OldestFirst = 'Oldest first',
}

export function ActivityTab({
  stats,
  targetedResourceTypeFilter,
  onResourceTypeFilterChange,
  showResourceTypeFilter = true,
}: {
  stats: { name: string; userTasks?: UserTask[] };
  targetedResourceTypeFilter: TaskResourceTypeFilter;
  onResourceTypeFilterChange: (filter: TaskResourceTypeFilter) => void;
  showResourceTypeFilter?: boolean;
}) {
  const tasks = stats.userTasks;
  const resourceTypeFilter = showResourceTypeFilter
    ? targetedResourceTypeFilter
    : 'All';

  const theme = useTheme();
  const queryClient = useQueryClient();
  const toastNotification = useToastNotifications();

  const [search, setSearch] = useState('');
  const [eventTypeFilter, setEventTypeFilter] =
    useState<EventTypeFilter>('All');
  const [dateOrder, setDateOrder] = useState<DateOrder>(DateOrder.NewestFirst);

  const [selected, setSelected] = useState<Record<string, boolean>>({});
  const [bulkError, setBulkError] = useState<string>('');
  const [bulkProcessing, setBulkProcessing] = useState(false);
  const [showBulkResolveDialog, setShowBulkResolveDialog] = useState(false);

  const [drawerTaskName, setDrawerTaskName] = useState<string>('');

  const filteredTasks = useMemo(() => {
    const q = search.trim().toLowerCase();
    const filtered = (tasks ?? []).filter(t => {
      const type = getTaskEventType(t);
      if (eventTypeFilter !== 'All' && type !== eventTypeFilter) {
        return false;
      }
      if (
        resourceTypeFilter !== 'All' &&
        getTaskResourceType(t) !== resourceTypeFilter
      ) {
        return false;
      }
      if (!q) {
        return true;
      }
      return (
        type.toLowerCase().includes(q) ||
        t.title?.toLowerCase().includes(q) ||
        t.issueType?.toLowerCase().includes(q) ||
        t.taskType?.toLowerCase().includes(q)
      );
    });

    filtered.sort((a, b) => {
      const at = Date.parse(a.lastStateChange || '');
      const bt = Date.parse(b.lastStateChange || '');
      const diff = (Number.isNaN(at) ? 0 : at) - (Number.isNaN(bt) ? 0 : bt);
      return dateOrder === DateOrder.NewestFirst ? -diff : diff;
    });

    return filtered;
  }, [tasks, search, eventTypeFilter, resourceTypeFilter, dateOrder]);

  const selectedNames = useMemo(
    () => filteredTasks.filter(t => selected[t.name]).map(t => t.name),
    [filteredTasks, selected]
  );

  const tableContextValue: ActivityTableContextValue = {
    selected,
    tasks: filteredTasks,
    disabled: bulkProcessing,
    onOpenDetails: setDrawerTaskName,
    onToggleOne: (name, checked) =>
      setSelected(prev => ({ ...prev, [name]: checked })),
    onToggleAll: checked =>
      setSelected(prev => {
        const next = { ...prev };
        for (const t of filteredTasks) {
          next[t.name] = checked;
        }
        return next;
      }),
  };

  async function markSelectedAsResolved() {
    setBulkError('');
    setBulkProcessing(true);
    const count = selectedNames.length;
    try {
      const results = await Promise.allSettled(
        selectedNames.map(name => integrationService.resolveUserTask(name))
      );

      const succeededNames = selectedNames.filter(
        (_, index) => results[index].status === 'fulfilled'
      );
      const failedResults = results.filter(
        result => result.status === 'rejected'
      ) as PromiseRejectedResult[];

      if (succeededNames.length > 0) {
        queryClient.setQueryData(
          ['integrationStats', stats.name],
          (prev: IntegrationWithSummary | undefined) => {
            if (!prev) {
              return prev;
            }
            const prevTasks = prev.userTasks ?? [];
            const nextTasks = prevTasks.filter(
              t => !succeededNames.includes(t.name)
            );
            const removedCount = prevTasks.length - nextTasks.length;
            return {
              ...prev,
              userTasks: nextTasks,
              unresolvedUserTasks: Math.max(
                0,
                prev.unresolvedUserTasks - removedCount
              ),
            };
          }
        );
      }

      setSelected(prev => {
        if (succeededNames.length === 0) {
          return prev;
        }
        const next = { ...prev };
        for (const name of succeededNames) {
          delete next[name];
        }
        return next;
      });

      if (succeededNames.length > 0) {
        toastNotification.add({
          severity: 'success',
          content: {
            title: `${succeededNames.length} ${pluralize(
              succeededNames.length,
              'issue'
            )} marked as resolved`,
            isAutoRemovable: true,
          },
        });
      }

      if (failedResults.length > 0) {
        const firstError =
          failedResults[0]?.reason?.message ||
          'Failed to mark some tasks as resolved';
        const errorMessage =
          failedResults.length === count
            ? firstError
            : `${failedResults.length} ${pluralize(
                failedResults.length,
                'issue'
              )} failed to resolve: ${firstError}`;

        setBulkError(errorMessage);
        toastNotification.add({
          severity: 'error',
          content: {
            title: 'Failed to mark issues as resolved',
            description: errorMessage,
            isAutoRemovable: true,
          },
        });
      }
    } finally {
      setBulkProcessing(false);
    }
  }

  async function confirmBulkResolve() {
    await markSelectedAsResolved();
    setShowBulkResolveDialog(false);
  }

  return (
    <>
      <Box>
        <Flex mb={3} alignItems="center" gap={2} flexWrap="wrap">
          <MenuButton
            buttonText={`Event Type: ${eventTypeFilter}`}
            onClear={
              eventTypeFilter === 'All'
                ? undefined
                : () => setEventTypeFilter('All')
            }
          >
            {(
              ['All', ...Object.values(TaskEventType)] as EventTypeFilter[]
            ).map(option => (
              <MenuItem key={option} onClick={() => setEventTypeFilter(option)}>
                <Flex alignItems="center" gap={2}>
                  <Box width="14px">
                    {option === eventTypeFilter && <Check size={14} />}
                  </Box>
                  <Text>{option}</Text>
                </Flex>
              </MenuItem>
            ))}
          </MenuButton>
          {showResourceTypeFilter && (
            <MenuButton
              buttonText={`Resource Type: ${targetedResourceTypeFilter}`}
              onClear={
                targetedResourceTypeFilter === 'All'
                  ? undefined
                  : () => onResourceTypeFilterChange('All')
              }
            >
              {taskResourceTypeFilters.map(filter => (
                <MenuItem
                  key={filter}
                  onClick={() => onResourceTypeFilterChange(filter)}
                >
                  <Flex alignItems="center" gap={2}>
                    <Box width="14px">
                      {filter === targetedResourceTypeFilter && (
                        <Check size={14} />
                      )}
                    </Box>
                    <Text>{filter}</Text>
                  </Flex>
                </MenuItem>
              ))}
            </MenuButton>
          )}
          <Box width="600px">
            <InputSearch
              searchValue={search}
              setSearchValue={setSearch}
              placeholder="Search..."
            />
          </Box>
        </Flex>
        {bulkError && (
          <Box mb={3}>
            <Danger dismissible onDismiss={() => setBulkError('')}>
              {bulkError}
            </Danger>
          </Box>
        )}
        <Flex alignItems="center" gap={2} mb={2}>
          {selectedNames.length > 0 && (
            <>
              <ButtonPrimaryBorder
                size="small"
                disabled={bulkProcessing}
                onClick={() => setShowBulkResolveDialog(true)}
                css={{ minWidth: '140px' }}
              >
                {bulkProcessing ? (
                  <Indicator size={14} color="text.muted" delay="none" />
                ) : (
                  'Mark as Resolved'
                )}
              </ButtonPrimaryBorder>
              <Text typography="body3" color="text.slightlyMuted">
                {selectedNames.length} selected
              </Text>
            </>
          )}
          <Box ml="auto">
            <MenuButton buttonText={`Date: ${dateOrder}`}>
              <MenuItem onClick={() => setDateOrder(DateOrder.NewestFirst)}>
                Newest first
              </MenuItem>
              <MenuItem onClick={() => setDateOrder(DateOrder.OldestFirst)}>
                Oldest first
              </MenuItem>
            </MenuButton>
          </Box>
        </Flex>
        <ActivityTableContext.Provider value={tableContextValue}>
          <TableWrapper>
            <DataTable<UserTask>
              data={filteredTasks}
              columns={COLUMNS}
              getRowId={task => task.name}
              emptyText="No issues found"
              row={{
                onClick: task => setDrawerTaskName(task.name),
                getStyle: task => {
                  if (task.name === drawerTaskName) {
                    return {
                      backgroundColor:
                        theme.colors.interactive.tonal.primary[0],
                      cursor: 'pointer',
                    };
                  }
                  return { cursor: 'pointer' };
                },
              }}
            />
          </TableWrapper>
        </ActivityTableContext.Provider>
      </Box>

      <Backdrop
        isVisible={Boolean(drawerTaskName)}
        onClick={() => setDrawerTaskName('')}
      />

      <FullHeightSlidingSidePanel
        slideFrom="right"
        isVisible={Boolean(drawerTaskName)}
        skipAnimation={false}
        panelWidth={560}
        zIndex={1000}
      >
        {drawerTaskName && (
          <UserTaskDrawer
            taskName={drawerTaskName}
            integrationName={stats.name}
            onClose={() => setDrawerTaskName('')}
            onResolved={() => {
              setDrawerTaskName('');
              setSelected(prev => ({ ...prev, [drawerTaskName]: false }));
            }}
          />
        )}
      </FullHeightSlidingSidePanel>
      {showBulkResolveDialog && (
        <MarkAsResolvedDialog
          confirmDisabled={selectedNames.length === 0}
          isProcessing={bulkProcessing}
          onCancel={() => {
            if (!bulkProcessing) {
              setShowBulkResolveDialog(false);
            }
          }}
          onConfirm={confirmBulkResolve}
        />
      )}
    </>
  );
}

type ActivityTableContextValue = {
  tasks: UserTask[];
  selected: Record<string, boolean>;
  disabled: boolean;
  onToggleAll: (checked: boolean) => void;
  onToggleOne: (name: string, checked: boolean) => void;
  onOpenDetails: (name: string) => void;
};

const ActivityTableContext = createContext<ActivityTableContextValue | null>(
  null
);

function useActivityTableContext(): ActivityTableContextValue {
  return useContext(ActivityTableContext)!;
}

function SelectAllHeader() {
  const { tasks, selected, onToggleAll, disabled } = useActivityTableContext();
  const allSelected = tasks.length > 0 && tasks.every(t => selected[t.name]);
  const someSelected = !allSelected && tasks.some(t => selected[t.name]);

  return (
    <ComposedCheckbox
      size="sm"
      aria-label="Select all visible issues"
      checked={allSelected ? true : someSelected ? 'indeterminate' : false}
      onCheckedChange={() => onToggleAll(!allSelected)}
      disabled={disabled || tasks.length === 0}
    />
  );
}

// we dont want the row click to happen if we are clicking the checkbox input so we stop the propagation
const stopRowClick: MouseEventHandler = e => e.stopPropagation();
function RowSelectCell({ row }: { row: Row<UserTask> }) {
  const { selected, onToggleOne, disabled } = useActivityTableContext();
  const task = row.original;
  return (
    <Flex alignItems="center" onClick={stopRowClick}>
      <ComposedCheckbox
        size="sm"
        aria-label={`Select issue ${task.title}`}
        checked={Boolean(selected[task.name])}
        onCheckedChange={details =>
          onToggleOne(task.name, details.checked === true)
        }
        disabled={disabled}
      />
    </Flex>
  );
}

function EventTypeCell({ row }: { row: Row<UserTask> }) {
  const type = getTaskEventType(row.original);
  return (
    <Flex alignItems="center" gap={2}>
      {type === 'Error' ? (
        <CircleCross size={16} color="error.main" />
      ) : (
        <Warning size={16} color="warning.main" />
      )}
      <Text typography="body3">{type}</Text>
    </Flex>
  );
}

function DescriptionCell({ row }: { row: Row<UserTask> }) {
  return <Text typography="body3">{row.original.title}</Text>;
}

function DateCell({ row }: { row: Row<UserTask> }) {
  return <>{formatRelativeDate(row.original.lastStateChange)}</>;
}

function DetailsCell({ row }: { row: Row<UserTask> }) {
  const { onOpenDetails } = useActivityTableContext();
  return (
    <Flex justifyContent="flex-end">
      <ButtonBorder
        size="small"
        onClick={e => {
          e.stopPropagation();
          onOpenDetails(row.original.name);
        }}
      >
        Details
      </ButtonBorder>
    </Flex>
  );
}

const COLUMNS: ColumnDef<UserTask>[] = [
  { id: 'select', header: SelectAllHeader, cell: RowSelectCell },
  { id: 'eventType', header: 'Event Type', cell: EventTypeCell },
  { id: 'title', header: 'Description', cell: DescriptionCell },
  { id: 'lastStateChange', header: 'Date', cell: DateCell },
  { id: 'details', header: () => null, cell: DetailsCell },
];

const TableWrapper = styled.div`
  table {
    thead tr {
      background-color: ${({ theme }) =>
        theme.type === 'light'
          ? theme.colors.levels.deep
          : theme.colors.levels.elevated};
    }

    thead > tr > th:first-child,
    tbody > tr > td:first-child {
      width: 1%;
      white-space: nowrap;
    }

    tbody > tr:first-child {
      border-top: none;
    }

    tbody tr {
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

const Backdrop = styled.div<{ isVisible: boolean }>`
  position: fixed;
  inset: 0;
  z-index: 999;
  background: rgba(0, 0, 0, 0.55);
  transition: opacity 0.15s ease-in-out;
  opacity: ${p => (p.isVisible ? 1 : 0)};
  pointer-events: ${p => (p.isVisible ? 'auto' : 'none')};
`;

const FullHeightSlidingSidePanel = styled(SlidingSidePanel)`
  top: 0;
`;

export enum TaskEventType {
  Error = 'Error',
  Warning = 'Warning',
}

export function getTaskEventType(task: { issueType?: string }): TaskEventType {
  const issue = (task.issueType || '').toLowerCase();
  if (issue.includes('disabled') || issue.includes('not-connecting')) {
    return TaskEventType.Warning;
  }
  return TaskEventType.Error;
}

function getTaskResourceType(task: {
  taskType?: string;
}): Exclude<TaskResourceTypeFilter, 'All'> | undefined {
  const taskType = (task.taskType || '').toLowerCase();
  if (taskType.includes('ec2')) {
    return 'EC2';
  }
  if (taskType.includes('rds')) {
    return 'RDS';
  }
  if (taskType.includes('eks')) {
    return 'EKS';
  }
}
