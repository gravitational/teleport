/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import { useMemo, useState } from 'react';
import styled from 'styled-components';

import { Box, ButtonBorder, ButtonIcon, Flex, H2, Text } from 'design';
import Table, { Cell } from 'design/DataTable';
import InputSearch from 'design/DataTable/InputSearch';
import { TableColumn } from 'design/DataTable/types';
import { displayDateTime } from 'design/datetime';
import { ArrowRight, CircleCross, Cross, Warning } from 'design/Icon';
import { Markdown } from 'shared/components/Markdown/Markdown';
import Select from 'shared/components/Select';

import { FeatureBox } from 'teleport/components/Layout';
import { SlidingSidePanel } from 'teleport/components/SlidingSidePanel';

export function UserTasks() {
  const [search, setSearch] = useState('');
  const [integrationFilter, setIntegrationFilter] =
    useState<IntegrationFilter>('ALL');
  const [visiblePages, setVisiblePages] = useState(1);
  const [selectedTaskName, setSelectedTaskName] = useState('');

  const openTasks = useMemo(
    () => MOCK_TASKS.filter(task => task.state === 'OPEN'),
    []
  );

  const integrationOptions = useMemo(
    () => [
      'ALL' as const,
      ...Array.from(new Set(openTasks.map(t => t.integration).filter(Boolean))),
    ],
    [openTasks]
  );

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    return openTasks.filter(task => {
      if (
        integrationFilter !== 'ALL' &&
        task.integration !== integrationFilter
      ) {
        return false;
      }
      if (!q) {
        return true;
      }
      return (
        task.name.toLowerCase().includes(q) ||
        task.issueType.toLowerCase().includes(q) ||
        task.issueTitle.toLowerCase().includes(q) ||
        task.taskType.toLowerCase().includes(q)
      );
    });
  }, [search, integrationFilter, openTasks]);

  const pageSize = 20;
  const visible = filtered.slice(0, pageSize * visiblePages);
  const hasMore = visible.length < filtered.length;

  const selectedTask = openTasks.find(t => t.name === selectedTaskName);

  return (
    <FeatureBox maxWidth="1800px" pt={3}>
      <Flex alignItems="center" justifyContent="space-between" mb={3}>
        <Box>
          <H2>User Tasks</H2>
        </Box>
      </Flex>

      <Box mt={3}>
        <Box mb={2} width="560px" maxWidth="100%">
          <InputSearch
            searchValue={search}
            setSearchValue={v => {
              setSearch(v);
              setVisiblePages(1);
            }}
            placeholder="Search task id, issue, type..."
          />
        </Box>
        <Flex gap={2} flexWrap="wrap" mb={3}>
          <Box width="220px">
            <Select
              size="small"
              value={{
                value: integrationFilter,
                label:
                  integrationFilter === 'ALL'
                    ? 'All Integrations'
                    : integrationFilter,
              }}
              options={integrationOptions.map(i => ({
                value: i,
                label: i === 'ALL' ? 'All Integrations' : i,
              }))}
              onChange={opt => {
                setIntegrationFilter(
                  (opt?.value as IntegrationFilter) ?? 'ALL'
                );
                setVisiblePages(1);
              }}
            />
          </Box>
        </Flex>

        <Flex gap={3} alignItems="stretch">
          <Box flex="1" minWidth={0}>
            <ListSurface>
              <HeaderRow>
                <HeaderCell>Issue</HeaderCell>
                <HeaderCell>Task Type</HeaderCell>
                <HeaderCell>Affected</HeaderCell>
                <HeaderCell>Integration</HeaderCell>
                <HeaderCell>Last State Change</HeaderCell>
                <HeaderCell />
              </HeaderRow>
              {visible.map(task => {
                const isActive = selectedTaskName === task.name;
                return (
                  <TaskRow
                    key={task.name}
                    isActive={isActive}
                    onClick={() => setSelectedTaskName(task.name)}
                  >
                    <TaskCell>
                      <Flex alignItems="flex-start" gap={2}>
                        {getTaskSeverity(task) === 'Error' ? (
                          <CircleCross size={16} color="error.main" />
                        ) : (
                          <Warning size={16} color="warning.main" />
                        )}
                        <Box>
                          <Text typography="body3" bold>
                            {task.issueTitle}
                          </Text>
                          <Text typography="body3" color="text.slightlyMuted">
                            {task.issueType}
                          </Text>
                        </Box>
                      </Flex>
                    </TaskCell>
                    <TaskCell>{task.taskTypeLabel}</TaskCell>
                    <TaskCell>{task.affected}</TaskCell>
                    <TaskCell>
                      {task.integration ? (
                        <MonoText>{task.integration}</MonoText>
                      ) : null}
                    </TaskCell>
                    <TaskCell>{formatRelative(task.lastStateChange)}</TaskCell>
                    <TaskCell>
                      <ButtonBorder
                        size="small"
                        onClick={e => {
                          e.stopPropagation();
                          setSelectedTaskName(task.name);
                        }}
                      >
                        Details
                      </ButtonBorder>
                    </TaskCell>
                  </TaskRow>
                );
              })}
              {visible.length === 0 && (
                <Box p={4}>
                  <Text typography="body3" color="text.slightlyMuted">
                    No tasks found.
                  </Text>
                </Box>
              )}
            </ListSurface>

            <Flex mt={3} alignItems="center" justifyContent="space-between">
              <Text typography="body3" color="text.slightlyMuted">
                Showing {visible.length} of {filtered.length} matching tasks
              </Text>
              {hasMore && (
                <ButtonBorder
                  size="small"
                  onClick={() => setVisiblePages(p => p + 1)}
                >
                  Load More
                </ButtonBorder>
              )}
            </Flex>
          </Box>
        </Flex>
      </Box>

      <Backdrop
        isVisible={Boolean(selectedTask)}
        onClick={() => setSelectedTaskName('')}
      />
      <FullHeightSlidingSidePanel
        slideFrom="right"
        isVisible={Boolean(selectedTask)}
        skipAnimation={false}
        panelWidth={620}
        zIndex={1000}
      >
        {selectedTask && (
          <TaskDetails
            task={selectedTask}
            onClose={() => setSelectedTaskName('')}
          />
        )}
      </FullHeightSlidingSidePanel>
    </FeatureBox>
  );
}

function TaskDetails({
  task,
  onClose,
}: {
  task: MockTask;
  onClose: () => void;
}) {
  const impactsTable = useMemo(
    () => makeImpactsTable(task.resources),
    [task.resources]
  );

  return (
    <PanelContainer>
      <Flex alignItems="flex-start" justifyContent="space-between" p={3}>
        <Box>
          <H2>{task.issueTitle}</H2>
          <Text typography="body3" color="text.slightlyMuted">
            {task.name}
          </Text>
        </Box>
        <ButtonIcon onClick={onClose} aria-label="Close">
          <Cross size="medium" />
        </ButtonIcon>
      </Flex>

      <PanelBody>
        <SectionTitle>Information</SectionTitle>
        <InfoRow label="State" value={task.state} />
        <InfoRow label="Severity" value={getTaskSeverity(task)} />
        <InfoRow label="Task Type" value={task.taskTypeLabel} />
        <InfoRow label="Issue Type" value={task.issueType} />
        <InfoRow label="Integration" value={task.integration} />
        <InfoRow label="Region" value={task.region} />
        <InfoRow
          label="Last State Change"
          value={displayDateTime(new Date(task.lastStateChange))}
        />

        <SectionTitle>How To Fix</SectionTitle>
        <Markdown text={task.description} enableLinks />

        <SectionTitle>
          Impacted Resources ({task.resources.length})
        </SectionTitle>
        <Table
          data={impactsTable.data}
          columns={impactsTable.columns}
          emptyText="No impacted resources"
        />

        <SectionTitle>Raw Error Output</SectionTitle>
        <ErrorOutput>{task.rawError}</ErrorOutput>
      </PanelBody>
    </PanelContainer>
  );
}

function InfoRow({ label, value }: { label: string; value: string }) {
  return (
    <Flex mb={1} gap={2}>
      <Text
        typography="body3"
        color="text.slightlyMuted"
        css={{ minWidth: '160px' }}
      >
        {label}:
      </Text>
      <Text typography="body3">{value}</Text>
    </Flex>
  );
}

type ResourceItem = {
  id: string;
  name: string;
  kind: 'ec2' | 'eks' | 'rds' | 'vm';
  url?: string;
};

type ResourceRow = {
  id: string;
  name: string;
  kind: string;
  url?: string;
};

function makeImpactsTable(resources: ResourceItem[]): {
  data: ResourceRow[];
  columns: TableColumn<ResourceRow>[];
} {
  return {
    data: resources.map(r => ({
      id: r.id,
      name: r.name,
      kind: r.kind.toUpperCase(),
      url: r.url,
    })),
    columns: [
      { key: 'id', headerText: 'ID' },
      { key: 'name', headerText: 'Name' },
      { key: 'kind', headerText: 'Type' },
      {
        altKey: 'url',
        headerText: 'Resource',
        render: row =>
          row.url ? (
            <Cell>
              <a href={row.url} target="_blank" rel="noreferrer">
                <Flex alignItems="center" gap={1}>
                  Open
                  <ArrowRight size={14} />
                </Flex>
              </a>
            </Cell>
          ) : (
            <Cell>-</Cell>
          ),
      },
    ],
  };
}

function formatRelative(dateString: string): string {
  const now = Date.now();
  const ts = Date.parse(dateString);
  if (Number.isNaN(ts)) {
    return dateString;
  }
  const diff = Math.max(0, now - ts);
  const mins = Math.floor(diff / 60000);
  if (mins < 1) {
    return 'just now';
  }
  if (mins < 60) {
    return `${mins}m ago`;
  }
  const hours = Math.floor(mins / 60);
  if (hours < 24) {
    return `${hours}h ago`;
  }
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

type TaskState = 'OPEN' | 'RESOLVED';
type IntegrationFilter = 'ALL' | string;

type MockTask = {
  name: string;
  state: TaskState;
  taskTypeLabel: string;
  taskType: string;
  issueType: string;
  issueTitle: string;
  integration: string;
  region: string;
  affected: number;
  lastStateChange: string;
  description: string;
  rawError: string;
  resources: ResourceItem[];
};

type TaskSeverity = 'Error' | 'Warning';

function getTaskSeverity(task: Pick<MockTask, 'issueType'>): TaskSeverity {
  const issue = task.issueType.toLowerCase();
  if (issue.includes('not-connecting') || issue.includes('disabled')) {
    return 'Warning';
  }
  return 'Error';
}

const sharedRawError = `<*> ERROR [UPDATER] Command failed. error: [
ERROR REPORT:
Original Error: *errors.errorString size of download (217917539 bytes) exceeds available disk space
Stack Trace:
github.com/gravitational/teleport/lib/autoupdate/agent/installer.go:325
github.com/gravitational/teleport/lib/autoupdate/agent/updater.go:1050
runtime/main.go:283 runtime.main
User Message: failed to install
failed to run commands: exit status 1
]`;

const MOCK_TASKS: MockTask[] = [
  {
    name: '6342cc16-c5d4-5ae0-b3fa-0e20a76f3a69',
    state: 'OPEN',
    taskTypeLabel: 'AWS EC2',
    taskType: 'discover-ec2',
    issueType: 'ec2-ssm-agent-not-registered',
    issueTitle: 'SSM Agent not registered',
    integration: '',
    region: 'eu-central-1',
    affected: 2,
    lastStateChange: '2026-02-20T15:59:00.000Z',
    description:
      'Auto enrolling EC2 instances requires the SSM Agent to be installed and running.\n\nCheck SSM Fleet Manager and ensure the instance has the `AmazonSSMManagedInstanceCore` policy attached.',
    rawError: sharedRawError,
    resources: [
      {
        id: 'i-003a23be4c3d13fa8',
        name: 'tener-tf-13022026-v1-target-1',
        kind: 'ec2',
        url: 'https://console.aws.amazon.com/ec2/home?region=eu-central-1#InstanceDetails:instanceId=i-003a23be4c3d13fa8',
      },
      {
        id: 'i-085159ed62c5364c2',
        name: 'tener-tf-13022026-v1-target-2',
        kind: 'ec2',
        url: 'https://console.aws.amazon.com/ec2/home?region=eu-central-1#InstanceDetails:instanceId=i-085159ed62c5364c2',
      },
    ],
  },
  {
    name: '7f90bbde-74ef-58a0-a1fc-9a08b44622b9',
    state: 'OPEN',
    taskTypeLabel: 'Azure VM',
    taskType: 'discover-azure-vm',
    issueType: 'azure-vm-not-running',
    issueTitle: 'Azure VM is not running',
    integration: 'tener-dev-9cab71ed-azure-oidc',
    region: 'westeurope',
    affected: 1,
    lastStateChange: '2026-02-19T16:59:00.000Z',
    description:
      'The VM is stopped or deallocated. Start the VM and retry enrollment on the next discovery cycle.',
    rawError:
      'AuthorizationFailed: The client does not have authorization to perform action `Microsoft.Compute/virtualMachines/start/action` on scope `/subscriptions/...`',
    resources: [{ id: 'vm-92a7', name: 'payments-vm-01', kind: 'vm' }],
  },
  {
    name: '896ad278-e7f2-4f2e-9c4d-5f9dbf3a9910',
    state: 'OPEN',
    taskTypeLabel: 'AWS EKS',
    taskType: 'discover-eks',
    issueType: 'eks-agent-not-connecting',
    issueTitle: 'Kube agent not connecting',
    integration: 'prod-aws-oidc',
    region: 'us-east-1',
    affected: 3,
    lastStateChange: '2026-02-20T15:35:00.000Z',
    description:
      'Teleport attempted to install the Helm chart, but the Kubernetes agent did not connect. Check StatefulSet logs for `teleport-kube-agent` in namespace `teleport-agent`.',
    rawError:
      'cluster enrollment failed: websocket: bad handshake\nrequest_id=8fbb5a cluster=platform-eks-01',
    resources: [
      { id: 'platform-eks-01', name: 'platform-eks-01', kind: 'eks' },
      { id: 'platform-eks-02', name: 'platform-eks-02', kind: 'eks' },
      { id: 'platform-eks-03', name: 'platform-eks-03', kind: 'eks' },
    ],
  },
  {
    name: '1bf7005a-5e72-4f7e-a7de-9d48e06d4f33',
    state: 'OPEN',
    taskTypeLabel: 'AWS RDS',
    taskType: 'discover-rds',
    issueType: 'rds-iam-auth-disabled',
    issueTitle: 'IAM auth is disabled',
    integration: 'prod-aws-oidc',
    region: 'us-west-2',
    affected: 2,
    lastStateChange: '2026-02-20T14:40:00.000Z',
    description:
      'Enable IAM database authentication on affected databases to allow Teleport enrollment and connectivity.',
    rawError:
      'rds enrollment blocked: IAM authentication is disabled for db instance `orders-db-01`',
    resources: [
      { id: 'orders-db-01', name: 'orders-db-01', kind: 'rds' },
      { id: 'billing-db-aurora', name: 'billing-db-aurora', kind: 'rds' },
    ],
  },
  {
    name: '4f8fda93-61de-4d1f-95c8-ae8be8a9d002',
    state: 'RESOLVED',
    taskTypeLabel: 'AWS EC2',
    taskType: 'discover-ec2',
    issueType: 'ec2-ssm-agent-connection-lost',
    issueTitle: 'SSM connection lost',
    integration: '',
    region: 'eu-central-1',
    affected: 1,
    lastStateChange: '2026-02-20T11:10:00.000Z',
    description:
      'The SSM agent lost connectivity. Verify outbound access to AWS SSM endpoints and instance profile permissions.',
    rawError:
      'GetCommandInvocation timeout: context deadline exceeded after 2m0s',
    resources: [{ id: 'i-0aa1134', name: 'tener-bastion-01', kind: 'ec2' }],
  },
  {
    name: '57d64ce6-3d5d-4114-a4d1-a0e5bde6fafe',
    state: 'OPEN',
    taskTypeLabel: 'AWS EC2',
    taskType: 'discover-ec2',
    issueType: 'ec2-ssm-script-failure',
    issueTitle: 'Install script failed',
    integration: 'dev-aws-oidc',
    region: 'us-east-2',
    affected: 4,
    lastStateChange: '2026-02-20T16:07:00.000Z',
    description:
      'The installer command failed on target hosts. Review invocation logs and verify free disk space, package manager health, and script permissions.',
    rawError: sharedRawError,
    resources: [
      { id: 'i-0392f1a', name: 'dev-api-01', kind: 'ec2' },
      { id: 'i-0392f1b', name: 'dev-api-02', kind: 'ec2' },
      { id: 'i-0392f1c', name: 'dev-api-03', kind: 'ec2' },
      { id: 'i-0392f1d', name: 'dev-api-04', kind: 'ec2' },
    ],
  },
  {
    name: '2c29216b-9c86-4f39-88cb-cf0c5329a35a',
    state: 'RESOLVED',
    taskTypeLabel: 'AWS EKS',
    taskType: 'discover-eks',
    issueType: 'eks-status-not-active',
    issueTitle: 'Cluster status not active',
    integration: 'staging-aws-oidc',
    region: 'us-west-1',
    affected: 1,
    lastStateChange: '2026-02-18T13:15:00.000Z',
    description:
      'Cluster must be ACTIVE before enrollment can complete. Check cluster lifecycle events and retry.',
    rawError: 'cluster state=CURRENTLY_UPDATING, expected ACTIVE',
    resources: [{ id: 'staging-eks-02', name: 'staging-eks-02', kind: 'eks' }],
  },
  {
    name: 'dc66de51-84c6-4f95-9ee7-36d7ba133f4f',
    state: 'OPEN',
    taskTypeLabel: 'Azure VM',
    taskType: 'discover-azure-vm',
    issueType: 'azure-vm-missing-run-commands-permission',
    issueTitle: 'Missing run command permissions',
    integration: 'tener-dev-9cab71ed-azure-oidc',
    region: 'eastus',
    affected: 2,
    lastStateChange: '2026-02-20T15:51:00.000Z',
    description:
      'Grant `runCommands/read` and `runCommands/write` permissions to the integration principal on the VM scope.',
    rawError:
      "AuthorizationFailed: does not have authorization to perform action 'Microsoft.Compute/virtualMachines/runCommands/write'",
    resources: [
      { id: 'vm-21aa', name: 'ingest-vm-01', kind: 'vm' },
      { id: 'vm-21ab', name: 'ingest-vm-02', kind: 'vm' },
    ],
  },
  {
    name: 'd25288fa-9ef9-4090-b5a1-89a370915985',
    state: 'OPEN',
    taskTypeLabel: 'AWS EC2',
    taskType: 'discover-ec2',
    issueType: 'ec2-ssm-invocation-failure',
    issueTitle: 'SSM invocation failed',
    integration: 'prod-aws-oidc',
    region: 'eu-west-1',
    affected: 2,
    lastStateChange: '2026-02-20T16:09:00.000Z',
    description:
      'Invocation failed before installation could complete. Verify document permissions, target selection, and command payload.',
    rawError:
      'Command invocations failed: exit code 127\n/bin/bash: line 2: error:: command not found',
    resources: [
      { id: 'i-0099c', name: 'telemetry-collector-01', kind: 'ec2' },
      { id: 'i-0099d', name: 'telemetry-collector-02', kind: 'ec2' },
    ],
  },
  {
    name: '19f7ac72-f8ca-4a7e-9ec0-9ed8bb4f1381',
    state: 'OPEN',
    taskTypeLabel: 'AWS EC2',
    taskType: 'discover-ec2',
    issueType: 'ec2-ssm-agent-not-running',
    issueTitle: 'SSM Agent is not running',
    integration: 'dev-aws-oidc',
    region: 'us-east-1',
    affected: 1,
    lastStateChange: '2026-02-20T16:10:00.000Z',
    description: 'Restart the SSM agent service and verify it stays healthy.',
    rawError: 'amazon-ssm-agent service status: inactive (dead)',
    resources: [{ id: 'i-01f001', name: 'orders-worker-01', kind: 'ec2' }],
  },
  {
    name: '1961f8ac-7448-42f1-96bc-d83d95a08952',
    state: 'OPEN',
    taskTypeLabel: 'AWS EKS',
    taskType: 'discover-eks',
    issueType: 'eks-authentication-mode-unsupported',
    issueTitle: 'Unsupported EKS authentication mode',
    integration: 'prod-aws-oidc',
    region: 'eu-central-1',
    affected: 2,
    lastStateChange: '2026-02-20T16:11:00.000Z',
    description: 'Adjust EKS authentication mode, then retry enrollment.',
    rawError: 'cluster auth mode CONFIG_MAP is unsupported for enrollment',
    resources: [
      { id: 'core-eks-01', name: 'core-eks-01', kind: 'eks' },
      { id: 'core-eks-02', name: 'core-eks-02', kind: 'eks' },
    ],
  },
  {
    name: '010809c2-bbed-4a74-a0af-03d3ce6f50f6',
    state: 'OPEN',
    taskTypeLabel: 'AWS RDS',
    taskType: 'discover-rds',
    issueType: 'rds-security-group-missing-rule',
    issueTitle: 'Required security group rule missing',
    integration: 'staging-aws-oidc',
    region: 'us-west-2',
    affected: 1,
    lastStateChange: '2026-02-20T16:12:00.000Z',
    description: 'Add required SG rules to permit enrollment traffic.',
    rawError: 'connection timeout: source sg rule missing on target database',
    resources: [
      { id: 'analytics-rds-01', name: 'analytics-rds-01', kind: 'rds' },
    ],
  },
  {
    name: 'ac2ddba3-3238-4f77-ab7d-e8138c56a70b',
    state: 'OPEN',
    taskTypeLabel: 'Azure VM',
    taskType: 'discover-azure-vm',
    issueType: 'azure-vm-agent-not-installed',
    issueTitle: 'Required VM agent is not installed',
    integration: 'tener-dev-9cab71ed-azure-oidc',
    region: 'eastus2',
    affected: 2,
    lastStateChange: '2026-02-20T16:13:00.000Z',
    description: 'Install required VM extension and retry run command.',
    rawError: 'run command failed: required extension not installed',
    resources: [
      { id: 'vm-31ca', name: 'reporting-vm-01', kind: 'vm' },
      { id: 'vm-31cb', name: 'reporting-vm-02', kind: 'vm' },
    ],
  },
  {
    name: 'ff872f2b-a3da-4ea5-a7d6-bee05e986f1d',
    state: 'OPEN',
    taskTypeLabel: 'AWS EC2',
    taskType: 'discover-ec2',
    issueType: 'ec2-ssm-command-timeout',
    issueTitle: 'SSM command timed out',
    integration: 'prod-aws-oidc',
    region: 'ap-southeast-1',
    affected: 1,
    lastStateChange: '2026-02-20T16:14:00.000Z',
    description: 'Command exceeded timeout during install step.',
    rawError: 'command timeout after 120s while installing package',
    resources: [{ id: 'i-02f120', name: 'edge-gateway-01', kind: 'ec2' }],
  },
  {
    name: '07ebf4e4-6d9f-4d7d-b58f-95cf13e39f7c',
    state: 'OPEN',
    taskTypeLabel: 'AWS EKS',
    taskType: 'discover-eks',
    issueType: 'eks-cluster-unreachable',
    issueTitle: 'EKS cluster endpoint unreachable',
    integration: 'staging-aws-oidc',
    region: 'us-east-2',
    affected: 1,
    lastStateChange: '2026-02-20T16:15:00.000Z',
    description: 'Ensure cluster endpoint is reachable from discovery runner.',
    rawError: 'dial tcp 10.32.1.20:443: i/o timeout',
    resources: [
      { id: 'payments-eks-01', name: 'payments-eks-01', kind: 'eks' },
    ],
  },
  {
    name: '41449980-ed8e-4e90-ab32-f1ee8a43f74f',
    state: 'OPEN',
    taskTypeLabel: 'AWS RDS',
    taskType: 'discover-rds',
    issueType: 'rds-instance-not-available',
    issueTitle: 'RDS instance not available',
    integration: 'dev-aws-oidc',
    region: 'eu-west-1',
    affected: 2,
    lastStateChange: '2026-02-20T16:16:00.000Z',
    description: 'Wait for instance availability and rerun discovery.',
    rawError: 'db instance status=modifying, expected available',
    resources: [
      { id: 'inventory-db-01', name: 'inventory-db-01', kind: 'rds' },
      { id: 'inventory-db-02', name: 'inventory-db-02', kind: 'rds' },
    ],
  },
  {
    name: '5bd9783b-bdb4-41f1-bc77-1e9dace50bc2',
    state: 'OPEN',
    taskTypeLabel: 'Azure VM',
    taskType: 'discover-azure-vm',
    issueType: 'azure-vm-not-running',
    issueTitle: 'Azure VM is not running',
    integration: 'azure-prod-oidc',
    region: 'centralus',
    affected: 1,
    lastStateChange: '2026-02-20T16:17:00.000Z',
    description: 'Start VM and confirm power state is running.',
    rawError: 'vm powerState=deallocated, expected running',
    resources: [{ id: 'vm-1001', name: 'search-vm-01', kind: 'vm' }],
  },
  {
    name: 'e5d0fbd2-c98a-4f13-ad83-99af74d7f92d',
    state: 'OPEN',
    taskTypeLabel: 'AWS EC2',
    taskType: 'discover-ec2',
    issueType: 'ec2-ssm-agent-not-registered',
    issueTitle: 'SSM Agent not registered',
    integration: '',
    region: 'eu-north-1',
    affected: 1,
    lastStateChange: '2026-02-20T16:18:00.000Z',
    description: 'Attach IAM profile and ensure SSM endpoint connectivity.',
    rawError: 'instance is not managed by SSM; no ping status available',
    resources: [{ id: 'i-08af220', name: 'ops-jumpbox-01', kind: 'ec2' }],
  },
  {
    name: '2390ef7a-b717-43a8-a9a1-5b278f426b87',
    state: 'OPEN',
    taskTypeLabel: 'AWS EKS',
    taskType: 'discover-eks',
    issueType: 'eks-agent-not-connecting',
    issueTitle: 'Kube agent not connecting',
    integration: 'prod-aws-oidc',
    region: 'us-west-1',
    affected: 1,
    lastStateChange: '2026-02-20T16:19:00.000Z',
    description: 'Inspect kube agent logs and network egress settings.',
    rawError: 'agent heartbeat missing for 10m; websocket disconnected',
    resources: [{ id: 'audit-eks-01', name: 'audit-eks-01', kind: 'eks' }],
  },
  {
    name: 'da7db429-60c0-497f-a95b-c7f4ba3aa37f',
    state: 'OPEN',
    taskTypeLabel: 'AWS RDS',
    taskType: 'discover-rds',
    issueType: 'rds-iam-auth-disabled',
    issueTitle: 'IAM auth is disabled',
    integration: 'shared-services-aws-oidc',
    region: 'ca-central-1',
    affected: 3,
    lastStateChange: '2026-02-20T16:20:00.000Z',
    description: 'Enable IAM auth for all listed RDS targets.',
    rawError: 'iam auth disabled on 3 targets; enrollment skipped',
    resources: [
      { id: 'shared-db-01', name: 'shared-db-01', kind: 'rds' },
      { id: 'shared-db-02', name: 'shared-db-02', kind: 'rds' },
      { id: 'shared-db-03', name: 'shared-db-03', kind: 'rds' },
    ],
  },
  {
    name: '0c3f4a67-f0f0-4f7a-b244-dcc2d7a61dc5',
    state: 'OPEN',
    taskTypeLabel: 'Azure VM',
    taskType: 'discover-azure-vm',
    issueType: 'azure-vm-network-not-reachable',
    issueTitle: 'Azure VM network unreachable',
    integration: 'azure-prod-oidc',
    region: 'westus2',
    affected: 2,
    lastStateChange: '2026-02-20T16:21:00.000Z',
    description: 'Fix VNET routing and NSG to allow discovery actions.',
    rawError: 'network probe failed: tcp connect timeout',
    resources: [
      { id: 'vm-1003', name: 'stream-vm-01', kind: 'vm' },
      { id: 'vm-1004', name: 'stream-vm-02', kind: 'vm' },
    ],
  },
  {
    name: 'c5ef0b73-f0d5-4d59-a60a-eb4c4afad153',
    state: 'OPEN',
    taskTypeLabel: 'AWS EC2',
    taskType: 'discover-ec2',
    issueType: 'ec2-ssm-script-failure',
    issueTitle: 'Install script failed',
    integration: 'shared-services-aws-oidc',
    region: 'us-east-1',
    affected: 2,
    lastStateChange: '2026-02-20T16:22:00.000Z',
    description: 'Fix script syntax and rerun enrollment command.',
    rawError:
      'bash: line 5: syntax error near unexpected token `}`\nexit status 2',
    resources: [
      { id: 'i-00ff11', name: 'batch-node-01', kind: 'ec2' },
      { id: 'i-00ff12', name: 'batch-node-02', kind: 'ec2' },
    ],
  },
];

const ListSurface = styled.div`
  border: 1px solid ${p => p.theme.colors.spotBackground[1]};
  border-radius: 10px;
  overflow: auto;
  min-width: 980px;
`;

const HeaderRow = styled.div`
  display: grid;
  grid-template-columns:
    minmax(320px, 2fr) 140px 90px minmax(260px, 2fr)
    140px 110px;
  align-items: center;
  border-bottom: 1px solid ${p => p.theme.colors.spotBackground[1]};
`;

const HeaderCell = styled.div`
  padding: 10px 12px;
  font-size: 12px;
  font-weight: 600;
  color: ${p => p.theme.colors.text.slightlyMuted};
`;

const TaskRow = styled.div<{ isActive: boolean }>`
  display: grid;
  grid-template-columns:
    minmax(320px, 2fr) 140px 90px minmax(260px, 2fr)
    140px 110px;
  align-items: center;
  min-height: 58px;
  border-bottom: 1px solid ${p => p.theme.colors.spotBackground[1]};
  cursor: pointer;
  background: ${p =>
    p.isActive ? p.theme.colors.interactive.tonal.primary[0] : 'transparent'};

  &:hover {
    background: ${p => p.theme.colors.interactive.tonal.primary[0]};
  }
`;

const TaskCell = styled.div`
  padding: 8px 12px;
  font-size: 14px;
  overflow: hidden;
`;

const MonoText = styled.span`
  font-family: monospace;
  font-size: 13px;
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

const PanelContainer = styled.div`
  height: 100%;
  display: flex;
  flex-direction: column;
  background: ${p => p.theme.colors.levels.sunken};
`;

const PanelBody = styled.div`
  overflow: auto;
  padding: 0 16px 16px;
`;

const SectionTitle = styled.h3`
  font-size: 18px;
  margin: 18px 0 10px;
`;

const ErrorOutput = styled.pre`
  background: #0a0f1c;
  color: #c4d2ff;
  border: 1px solid #24314d;
  border-radius: 8px;
  padding: 12px;
  overflow: auto;
  font-size: 12px;
  line-height: 1.5;
`;
