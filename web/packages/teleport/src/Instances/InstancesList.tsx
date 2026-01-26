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

import { Link } from 'react-router-dom';
import styled from 'styled-components';

import { Box, Flex, Text } from 'design';
import { Danger } from 'design/Alert';
import { ButtonBorder } from 'design/Button';
import Table, { Cell } from 'design/DataTable';
import * as Icons from 'design/Icon';
import { Indicator } from 'design/Indicator';
import { HoverTooltip } from 'design/Tooltip';
import { CopyButton } from 'shared/components/CopyButton/CopyButton';
import { useInfiniteScroll } from 'shared/hooks';

import cfg from 'teleport/config';
import { UnifiedInstance } from 'teleport/services/instances/types';

type TableInstance = {
  name: string;
  version: string;
  type: 'instance' | 'bot_instance';
  original: UnifiedInstance;
};

export function InstancesList(props: {
  data: UnifiedInstance[];
  isLoading: boolean;
  isFetchingNextPage: boolean;
  error: Error | null;
  hasNextPage: boolean;
  sortField: string;
  sortDir: string;
  onSortChanged: (sortField: string, sortDir: string) => void;
  onLoadNextPage: () => void;
}) {
  const {
    data,
    isLoading,
    isFetchingNextPage,
    error,
    hasNextPage,
    sortField,
    sortDir,
    onSortChanged,
    onLoadNextPage,
  } = props;

  // Normalize the data
  const tableData: TableInstance[] = data.map(instance => ({
    name:
      instance.type === 'instance'
        ? instance.instance?.name || instance.id
        : instance.botInstance?.name || '',
    version:
      instance.type === 'instance'
        ? instance.instance?.version || ''
        : instance.botInstance?.version || '',
    type: instance.type,
    original: instance,
  }));

  const { setTrigger } = useInfiniteScroll({
    fetch: async () => {
      if (hasNextPage && !isFetchingNextPage) {
        onLoadNextPage();
      }
    },
  });

  if (isLoading) {
    return (
      <Box textAlign="center" m={10}>
        <Indicator />
      </Box>
    );
  }

  if (error) {
    return (
      <Danger m={3} details={error.message}>
        Failed to fetch instances
      </Danger>
    );
  }

  if (!data || data.length === 0) {
    return (
      <Box textAlign="center" m={10}>
        <Text typography="h3" mb={3}>
          No instances found
        </Text>
      </Box>
    );
  }

  return (
    <Box>
      <StyledTable
        data={tableData}
        columns={[
          {
            key: 'name',
            headerText: 'Host/Bot Name',
            isSortable: true,
            render: (row: TableInstance) => (
              <NameCell instance={row.original} />
            ),
          },
          {
            key: 'version',
            headerText: 'Version',
            isSortable: true,
            render: (row: TableInstance) => <Cell>{row.version}</Cell>,
          },
          {
            key: 'type',
            headerText: 'Type',
            isSortable: true,
            render: (row: TableInstance) => (
              <Cell>
                {row.type === 'instance' ? 'Instance' : 'Bot Instance'}
              </Cell>
            ),
          },
          {
            altKey: 'services',
            headerText: 'Services',
            render: (row: TableInstance) => (
              <ServicesCell instance={row.original} />
            ),
          },
          {
            altKey: 'upgrader',
            headerText: 'Upgrader',
            render: (row: TableInstance) => {
              const upgraderType =
                row.type === 'instance'
                  ? row.original.instance?.upgrader?.type
                  : undefined;
              return <UpgraderCell upgrader={upgraderType} />;
            },
          },
          {
            altKey: 'upgrader-group',
            headerText: 'Upgrader Group',
            render: (row: TableInstance) => {
              const group =
                row.type === 'instance'
                  ? row.original.instance?.upgrader?.group
                  : undefined;
              return <Cell>{group || ''}</Cell>;
            },
          },
        ]}
        emptyText="No instances found"
        customSort={{
          fieldName: sortField,
          dir: sortDir === 'DESC' ? 'DESC' : 'ASC',
          onSort: sort => {
            onSortChanged(sort.fieldName, sort.dir);
          },
        }}
      />
      <Box ref={setTrigger} height="40px" />
      {isFetchingNextPage && (
        <Box textAlign="center">
          <Indicator size={32} delay="none" />
        </Box>
      )}
    </Box>
  );
}

const StyledTable = styled(Table)`
  thead > tr > th {
    color: ${props => props.theme.colors.text.slightlyMuted};
  }
` as typeof Table;

function NameCell({ instance }: { instance: UnifiedInstance }) {
  const name =
    instance.type === 'instance'
      ? instance.instance?.name || instance.id // Use the id as the name in case it doesn't have a friendly name
      : instance.botInstance?.name;

  return (
    <Cell>
      {name && <Text>{name}</Text>}
      <IdContainer>
        <HoverTooltip tipContent={instance.id}>
          <IdText>{instance.id.substring(0, 7)}</IdText>
        </HoverTooltip>
        <CopyButtonWrapper>
          <CopyButton value={instance.id} tooltip="Copy instance ID" />
        </CopyButtonWrapper>
      </IdContainer>
    </Cell>
  );
}

/**
 * UpgraderCell displays the upgrader in a more readable way with styling
 */
function UpgraderCell({ upgrader }: { upgrader: string | undefined }) {
  if (!upgrader || upgrader === '') {
    return (
      <Cell>
        <Text color="editor.abbey">None</Text>
      </Cell>
    );
  }

  if (upgrader === 'unit-updater') {
    return (
      <Cell>
        <Text color="editor.sunflower">Unit Updater (legacy)</Text>
      </Cell>
    );
  }

  if (upgrader === 'systemd-unit-updater') {
    return (
      <Cell>
        <Text>Systemd Unit Updater</Text>
      </Cell>
    );
  }

  if (upgrader === 'kube-updater') {
    return (
      <Cell>
        <Text>Kubernetes</Text>
      </Cell>
    );
  }

  // This normally shouldn't happen, but in case it's none of the expected values, and it's also not empty, just display whatever it is as is
  return (
    <Cell>
      <Text>{upgrader}</Text>
    </Cell>
  );
}

function ServicesCell({ instance }: { instance: UnifiedInstance }) {
  // For bot instances, we don't list services in this table. Instead, we deeplink to the bot instance dashboard page with this
  // particular bot instance filtered for and selected
  if (instance.type === 'bot_instance') {
    const query = `spec.instance_id == "${instance.id}"`;
    const botName = instance.botInstance.name;
    const url = cfg.getBotInstancesRoute({
      query,
      isAdvancedQuery: true,
      selectedItemId: `${botName}/${instance.id}`,
    });

    return (
      <Cell>
        <Link to={url}>
          <ButtonBorder size="small" px={2} py={1}>
            Services <Icons.ArrowSquareIn size="small" ml={1} />
          </ButtonBorder>
        </Link>
      </Cell>
    );
  }

  const services = instance.instance?.services || [];

  return (
    <Cell>
      <Flex gap={2}>
        {services.map(service => {
          const IconComponent = getServiceIcon(service);
          const displayName = getServiceDisplayName(service);
          return (
            <HoverTooltip key={service} tipContent={displayName}>
              <Box>
                <IconComponent size="medium" />
              </Box>
            </HoverTooltip>
          );
        })}
      </Flex>
    </Cell>
  );
}

function getServiceIcon(service: string): React.ComponentType<any> {
  const serviceMap: Record<string, React.ComponentType<any>> = {
    node: Icons.Server,
    kube: Icons.Kubernetes,
    app: Icons.Application,
    db: Icons.Database,
    windowsdesktop: Icons.Desktop,
    proxy: Icons.Network,
    auth: Icons.Keypair,
  };

  return serviceMap[service.toLowerCase()] || Icons.Server;
}

function getServiceDisplayName(service: string): string {
  const displayNames: Record<string, string> = {
    node: 'SSH Server',
    kube: 'Kubernetes',
    app: 'Application',
    db: 'Database',
    windowsdesktop: 'Windows Desktop',
    proxy: 'Proxy',
    auth: 'Auth',
  };

  return displayNames[service.toLowerCase()] || service;
}

const IdContainer = styled(Box)`
  display: inline-flex;
  align-items: center;
  gap: ${props => props.theme.space[1]}px;
`;

const IdText = styled(Text)`
  color: ${props => props.theme.colors.text.muted};
  font-size: ${props => props.theme.fontSizes[1]}px;
  font-family: ${props => props.theme.fonts.mono};
`;

const CopyButtonWrapper = styled(Box)`
  display: inline-flex;
  align-items: center;
  opacity: 0;

  tr:hover & {
    opacity: 1;
  }
`;
