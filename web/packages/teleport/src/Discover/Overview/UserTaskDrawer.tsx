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

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useMemo } from 'react';
import styled from 'styled-components';

import { Box, ButtonIcon, Flex, H2, H3, Indicator, Link, Text } from 'design';
import { Danger } from 'design/Alert';
import { ButtonBorder, ButtonPrimary } from 'design/Button';
import Table, { Cell } from 'design/DataTable';
import { TableColumn } from 'design/DataTable/types';
import { displayDateTime } from 'design/datetime';
import { CircleCross, Cross, Link as LinkIcon, Warning } from 'design/Icon';
import { LabelButtonWithIcon } from 'design/Label/LabelButtonWithIcon';
import { P } from 'design/Text/Text';
import { Markdown } from 'shared/components/Markdown/Markdown';
import { useToastNotifications } from 'shared/components/ToastNotification';
import { getErrorMessage } from 'shared/utils/error';

import {
  integrationService,
  type DiscoverEc2Instance,
  type DiscoverEksCluster,
  type DiscoverRdsDatabase,
  type IntegrationWithSummary,
  type UserTaskDetail,
} from 'teleport/services/integrations';

import { getTaskEventType } from './ActivityTab';

export function UserTaskDrawer(props: {
  taskName: string;
  integrationName: string;
  onClose: () => void;
  onResolved: () => void | Promise<void>;
}) {
  const { taskName, integrationName, onClose, onResolved } = props;
  const queryClient = useQueryClient();
  const toastNotification = useToastNotifications();

  const {
    data: task,
    isLoading,
    isError,
    error,
    refetch,
  } = useQuery({
    queryKey: ['userTask', taskName],
    queryFn: ({ signal }) => integrationService.fetchUserTask(taskName, signal),
    enabled: Boolean(taskName),
  });

  const resolveTask = useMutation({
    mutationFn: (name: string) => integrationService.resolveUserTask(name),
    onSuccess: async (_, resolvedName) => {
      queryClient.setQueryData(
        ['integrationStats', integrationName],
        (prev: IntegrationWithSummary | undefined) => {
          if (!prev) {
            return prev;
          }
          const prevTasks = prev.userTasks ?? [];
          const nextTasks = prevTasks.filter(t => t.name !== resolvedName);
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

      const taskTitle = task?.title || resolvedName;
      toastNotification.add({
        severity: 'success',
        content: {
          title: `Issue "${taskTitle}" marked as resolved`,
          isAutoRemovable: true,
        },
      });

      await onResolved();
    },
    onError: err => {
      const errorMessage = getErrorMessage(err);
      toastNotification.add({
        severity: 'error',
        content: {
          title: 'Failed to resolve issue',
          description: errorMessage,
          isAutoRemovable: true,
        },
      });
    },
  });

  const isProcessing = resolveTask.isPending;

  return (
    <Container>
      <Header>
        <Flex alignItems="flex-start" gap={2} flex="1" minWidth={0}>
          <Box minWidth={0}>
            <TitleText title={task?.title || taskName}>
              {task?.title || taskName}
            </TitleText>
          </Box>
          {task && <TaskTypePill task={task} />}
        </Flex>
        <ButtonIcon onClick={onClose} aria-label="Close">
          <Cross size="medium" />
        </ButtonIcon>
      </Header>

      <Divider />

      <Body>
        {isLoading && (
          <Box textAlign="center" mt={6}>
            <Indicator />
          </Box>
        )}

        {isError && (
          <Box p={4}>
            <Danger>{error?.message || 'Failed to load issue details'}</Danger>
            <Box mt={3}>
              <ButtonBorder size="small" onClick={() => refetch()}>
                Retry
              </ButtonBorder>
            </Box>
          </Box>
        )}

        {task && <DetailsTab task={task} />}
      </Body>

      {/* TODO (avatus) use the new loading button when chakra gets in */}
      <Footer>
        <ButtonPrimary
          onClick={() => resolveTask.mutate(taskName)}
          disabled={isProcessing}
          css={{ minWidth: '160px' }}
        >
          {isProcessing ? (
            <Indicator size={16} color="text.muted" delay="none" />
          ) : (
            'Mark as Resolved'
          )}
        </ButtonPrimary>
      </Footer>
    </Container>
  );
}

function TaskTypePill({ task }: { task: UserTaskDetail }) {
  const eventType = getTaskEventType(task);
  const IconLeft = eventType === 'Error' ? CircleCross : Warning;

  return (
    <LabelButtonWithIcon
      IconLeft={IconLeft}
      kind={eventType === 'Error' ? 'outline-danger' : 'outline-warning'}
    >
      {eventType}
    </LabelButtonWithIcon>
  );
}

function DetailsTab({ task }: { task: UserTaskDetail }) {
  const info = useMemo(() => getTaskInfo(task), [task]);
  const impacted = useMemo(() => getImpacts(task), [task]);
  const impactsTable = useMemo(() => makeImpactsTable(impacted), [impacted]);

  return (
    <Box px={4} pb={4}>
      {/* TODO(avatus): add logs tab here when they are implemented. */}
      <H3 mt={3}>Information</H3>
      <InfoRow label="Type" value={getTaskEventType(task)} />
      <InfoRow label="Integration" value={task.integration} />
      <InfoRow label="Resource Type" value={info.resourceType} />
      <InfoRow label="Region" value={info.region} />
      <InfoRow label="Account" value={info.account} />
      <InfoRow label="Last updated" value={formatDate(task.lastStateChange)} />
      <Divider />

      <H3 mt={3}>Details</H3>
      <Markdown text={task.description || ''} enableLinks />

      {impacted.count > 0 && (
        <>
          <H3 mt={4}>Impacted resources ({impacted.count})</H3>
          <Table
            data={impactsTable.data}
            columns={impactsTable.columns}
            emptyText="No impacted resources"
          />
        </>
      )}

      <H3 mt={4}>Mark as Resolved</H3>
      <P>
        This issue will reappear if the underlying problem is not fixed.
        Teleport will automatically retry enrolling these resources in the next
        discovery scan.
      </P>
    </Box>
  );
}

function InfoRow({ label, value }: { label: string; value?: string }) {
  return (
    <Flex mb={1} alignItems="flex-start" gap={2}>
      <Text
        typography="body3"
        color="text.slightlyMuted"
        css={{ minWidth: '140px' }}
      >
        {label}:
      </Text>
      <Text typography="body3">{value || '-'}</Text>
    </Flex>
  );
}

function formatDate(value?: string): string {
  if (!value) {
    return '-';
  }
  const ts = Date.parse(value);
  if (Number.isNaN(ts)) {
    return value;
  }
  return displayDateTime(new Date(ts));
}

function getTaskInfo(task: UserTaskDetail): {
  resourceType: string;
  region?: string;
  account?: string;
} {
  const taskType = (task.taskType || '').toLowerCase();
  if (taskType.includes('ec2')) {
    return {
      resourceType: 'EC2',
      region: task.discoverEc2?.region,
      account: task.discoverEc2?.account_id,
    };
  }
  if (taskType.includes('eks')) {
    return {
      resourceType: 'EKS',
      region: task.discoverEks?.region,
      account: task.discoverEks?.account_id,
    };
  }
  return {
    resourceType: 'RDS',
    region: task.discoverRds?.region,
    account: task.discoverRds?.account_id,
  };
}

type ImpactRow = {
  id: string;
  name?: string;
  resourceUrl?: string;
  invocationUrl?: string;
  kind: 'ec2' | 'eks' | 'rds';
};

function getImpacts(task: UserTaskDetail): {
  rows: ImpactRow[];
  count: number;
} {
  const taskType = (task.taskType || '').toLowerCase();

  if (taskType.includes('ec2')) {
    const instances = task.discoverEc2?.instances ?? {};
    const rows = Object.keys(instances).map(key => {
      const inst = instances[key] as DiscoverEc2Instance;
      return {
        id: inst?.instance_id || key,
        name: inst?.name,
        resourceUrl: inst?.resourceUrl,
        invocationUrl: inst?.invocation_url,
        kind: 'ec2' as const,
      };
    });
    return { rows, count: rows.length };
  }

  if (taskType.includes('eks')) {
    const clusters = task.discoverEks?.clusters ?? {};
    const rows = Object.keys(clusters).map(key => {
      const c = clusters[key] as DiscoverEksCluster;
      return {
        id: c?.name || key,
        name: c?.name,
        resourceUrl: c?.resourceUrl,
        kind: 'eks' as const,
      };
    });
    return { rows, count: rows.length };
  }

  const databases = task.discoverRds?.databases ?? {};
  const rows = Object.keys(databases).map(key => {
    const d = databases[key] as DiscoverRdsDatabase;
    return {
      id: d?.name || key,
      name: d?.name,
      resourceUrl: d?.resourceUrl,
      kind: 'rds' as const,
    };
  });
  return { rows, count: rows.length };
}

function makeImpactsTable(impacted: { rows: ImpactRow[] }): {
  columns: TableColumn<ImpactRow>[];
  data: ImpactRow[];
} {
  const columns: TableColumn<ImpactRow>[] = [
    {
      key: 'id',
      headerText: 'Resource',
      render: row => {
        if (row.resourceUrl) {
          return (
            <Cell>
              <Flex flexDirection="column">
                <Link href={row.resourceUrl} target="_blank">
                  {row.id}
                </Link>
                {row.name && row.name !== row.id && (
                  <Text typography="body3" color="text.slightlyMuted">
                    {row.name}
                  </Text>
                )}
              </Flex>
            </Cell>
          );
        }
        return (
          <Cell>
            <Flex flexDirection="column">
              <Text typography="body3">{row.id}</Text>
              {row.name && row.name !== row.id && (
                <Text typography="body3" color="text.slightlyMuted">
                  {row.name}
                </Text>
              )}
            </Flex>
          </Cell>
        );
      },
    },
  ];

  const hasInvocation = impacted.rows.some(r => Boolean(r.invocationUrl));
  if (hasInvocation) {
    columns.push({
      altKey: 'invocation',
      headerText: 'Invocation',
      render: row =>
        row.invocationUrl ? (
          <Cell align="center">
            <Link href={row.invocationUrl} target="_blank">
              <InvocationButton
                intent="neutral"
                aria-label="Open invocation link"
              >
                <LinkIcon size="small" />
              </InvocationButton>
            </Link>
          </Cell>
        ) : (
          <Cell />
        ),
    });
  }

  return { columns, data: impacted.rows };
}

const Container = styled.section`
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
`;

const Header = styled(Flex)`
  align-items: flex-start;
  justify-content: space-between;
  gap: ${p => p.theme.space[2]}px;
  padding: ${p => p.theme.space[3]}px ${p => p.theme.space[4]}px;
  flex-shrink: 0;
`;

const TitleText = styled(H2)`
  margin: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
`;

const Divider = styled.div`
  height: 1px;
  flex-shrink: 0;
  background-color: ${p => p.theme.colors.interactive.tonal.neutral[0]};
`;

const Body = styled.div`
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
  overflow: auto;
`;

const Footer = styled(Flex)`
  align-items: center;
  border-top: 1px solid ${p => p.theme.colors.interactive.tonal.neutral[0]};
  padding: ${p => p.theme.space[3]}px ${p => p.theme.space[4]}px;
  flex-shrink: 0;
`;

const InvocationButton = styled(ButtonBorder)`
  width: 32px;
  height: 32px;
  padding: 0;
`;
