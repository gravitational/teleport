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

import { useQueryClient } from '@tanstack/react-query';
import {
  useMemo,
  useState,
  type ChangeEvent,
  type MouseEventHandler,
} from 'react';
import styled, { useTheme } from 'styled-components';

import { Box, Flex, Indicator, Text } from 'design';
import { Danger } from 'design/Alert';
import { ButtonBorder, ButtonPrimaryBorder } from 'design/Button';
import { CheckboxInput } from 'design/Checkbox';
import Table, { Cell } from 'design/DataTable';
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
enum DateOrder {
  NewestFirst = 'Newest first',
  OldestFirst = 'Oldest first',
}

export function ActivityTab({
  stats,
}: {
  stats: { name: string; userTasks?: UserTask[] };
}) {
  const tasks = stats.userTasks;

  const theme = useTheme();
  const queryClient = useQueryClient();
  const toastNotification = useToastNotifications();

  // we dont want the row click to happen if we are clicking the checkbox input so we stop the propagation
  const stopRowClick: MouseEventHandler = e => e.stopPropagation();

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
  }, [tasks, search, eventTypeFilter, dateOrder]);

  const selectedNames = useMemo(
    () => Object.keys(selected).filter(k => selected[k]),
    [selected]
  );

  function handleEventTypeFilterChange(newFilter: EventTypeFilter) {
    setEventTypeFilter(newFilter);
    if (newFilter === 'All') {
      return;
    }
    setSelected(prev => {
      const next = { ...prev };
      for (const task of tasks ?? []) {
        if (prev[task.name] && getTaskEventType(task) !== newFilter) {
          delete next[task.name];
        }
      }
      return next;
    });
  }

  const allVisibleSelected =
    filteredTasks.length > 0 &&
    filteredTasks.every(t => Boolean(selected[t.name]));

  const someVisibleSelected =
    filteredTasks.some(t => Boolean(selected[t.name])) && !allVisibleSelected;

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
          <Box width="600px">
            <InputSearch
              searchValue={search}
              setSearchValue={setSearch}
              placeholder="Search..."
            />
          </Box>
        </Flex>
        <Flex mb={3} alignItems="center" gap={2} flexWrap="wrap">
          <MenuButton buttonText={`Event Type: ${eventTypeFilter}`}>
            {(
              ['All', ...Object.values(TaskEventType)] as EventTypeFilter[]
            ).map(option => (
              <MenuItem
                key={option}
                onClick={() => handleEventTypeFilterChange(option)}
              >
                <Flex alignItems="center" gap={2}>
                  <Box width="14px">
                    {option === eventTypeFilter && <Check size={14} />}
                  </Box>
                  <Text>{option}</Text>
                </Flex>
              </MenuItem>
            ))}
          </MenuButton>
          <MenuButton buttonText={`Date: ${dateOrder}`}>
            <MenuItem onClick={() => setDateOrder(DateOrder.NewestFirst)}>
              Newest first
            </MenuItem>
            <MenuItem onClick={() => setDateOrder(DateOrder.OldestFirst)}>
              Oldest first
            </MenuItem>
          </MenuButton>
        </Flex>
        {bulkError && (
          <Box mb={3}>
            <Danger dismissible onDismiss={() => setBulkError('')}>
              {bulkError}
            </Danger>
          </Box>
        )}
        <Flex alignItems="center" gap={2} mb={2}>
          <Box>
            <CheckboxInput
              checked={allVisibleSelected}
              disabled={bulkProcessing}
              // we dont support indeterminate via props so we can set via ref in a wrapper
              ref={node => {
                if (node) {
                  node.indeterminate = someVisibleSelected;
                }
              }}
              onChange={e => {
                const checked = e.currentTarget.checked;
                setSelected(prev => {
                  const next = { ...prev };
                  for (const t of filteredTasks) {
                    next[t.name] = checked;
                  }
                  return next;
                });
              }}
              aria-label="Select all visible issues"
            />
          </Box>
          <ButtonPrimaryBorder
            size="small"
            disabled={bulkProcessing || selectedNames.length === 0}
            onClick={() => setShowBulkResolveDialog(true)}
            css={{ minWidth: '140px' }}
          >
            {bulkProcessing ? (
              <Indicator size={14} color="text.muted" delay="none" />
            ) : (
              'Mark as Resolved'
            )}
          </ButtonPrimaryBorder>
          {selectedNames.length > 0 && (
            <Text typography="body3" color="text.slightlyMuted">
              {selectedNames.length} selected
            </Text>
          )}
        </Flex>
        <Table<UserTask>
          className={filteredTasks.length === 0 ? 'activity-empty' : undefined}
          hideEmptyIcon
          data={filteredTasks}
          emptyText=" "
          emptyHint="No issues found"
          row={{
            onClick: row => setDrawerTaskName(row.name),
            getStyle: row => {
              if (row.name === drawerTaskName) {
                return {
                  backgroundColor: theme.colors.interactive.tonal.primary[0],
                  cursor: 'pointer',
                };
              }
              return { cursor: 'pointer' };
            },
          }}
          columns={[
            {
              altKey: 'select',
              headerText: '',
              render: task => (
                <Cell width="20px">
                  <Flex
                    alignItems="center"
                    justifyContent="center"
                    onMouseDown={stopRowClick}
                    onClick={stopRowClick}
                  >
                    <CheckboxInput
                      checked={Boolean(selected[task.name])}
                      disabled={bulkProcessing}
                      onChange={(e: ChangeEvent<HTMLInputElement>) => {
                        e.stopPropagation();
                        const checked = e.currentTarget.checked;
                        setSelected(prev => ({
                          ...prev,
                          [task.name]: checked,
                        }));
                      }}
                      aria-label={`Select issue ${task.title}`}
                    />
                  </Flex>
                </Cell>
              ),
            },
            {
              altKey: 'eventType',
              headerText: 'Event Type',
              render: task => {
                const type = getTaskEventType(task);
                return (
                  <Cell>
                    <Flex alignItems="center" gap={2}>
                      {type === 'Error' ? (
                        <CircleCross size={16} color="error.main" />
                      ) : (
                        <Warning size={16} color="warning.main" />
                      )}
                      <Text typography="body3">{type}</Text>
                    </Flex>
                  </Cell>
                );
              },
            },
            {
              key: 'title',
              headerText: 'Description',
              render: task => (
                <Cell>
                  <Text typography="body3">{task.title}</Text>
                </Cell>
              ),
            },
            {
              key: 'lastStateChange',
              headerText: 'Date',
              render: task => (
                <Cell>{formatRelativeDate(task.lastStateChange)}</Cell>
              ),
            },
            {
              altKey: 'details',
              headerText: '',
              render: task => (
                <Cell align="right">
                  <ButtonBorder
                    size="small"
                    onClick={e => {
                      e.stopPropagation();
                      setDrawerTaskName(task.name);
                    }}
                  >
                    Details
                  </ButtonBorder>
                </Cell>
              ),
            },
          ]}
        />
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
