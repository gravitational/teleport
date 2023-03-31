import React from 'react';
import styled from 'styled-components';

import { Box, Flex } from 'design';
import Table, { Cell, TextCell } from 'design/DataTable';
import { MenuButton, MenuItem } from 'shared/components/MenuAction';

import { Plugin } from '../services/plugins';

import { pluginTypeMap } from './data';

import { PluginIcon } from './PluginIcon';

type Props = {
  plugins: Plugin[];
  onDelete: (p: Plugin) => void;
};

export function PluginList({ plugins = [], onDelete }: Props) {
  return (
    <Table
      data={plugins}
      columns={[
        {
          key: 'type',
          headerText: 'Integration',
          isSortable: true,
          render: plugin => (
            <Cell>
              <Flex alignItems="center">
                <PluginIcon size={18} pr={2} type={plugin.type} />
                {pluginTypeMap[plugin.type]?.name ?? plugin.type}
              </Flex>
            </Cell>
          ),
        },
        {
          key: 'details',
          headerText: 'Details',
        },
        {
          key: 'status',
          headerText: 'Status',
          isSortable: true,
          render: plugin => (
            <Cell>
              <Flex alignItems="center">
                <StatusLight status={plugin.status.code} />
                <TextCell data={plugin.status.code} />
              </Flex>
            </Cell>
          ),
        },
        {
          altKey: 'options-btn',
          render: plugin => <ActionCell onDelete={() => onDelete(plugin)} />,
        },
      ]}
      emptyText="No Integrations Found"
    />
  );
}

const ActionCell = ({ onDelete }: { onDelete: () => void }) => {
  return (
    <Cell align="right">
      <MenuButton>
        <MenuItem onClick={onDelete}>Delete...</MenuItem>
      </MenuButton>
    </Cell>
  );
};

const StatusLight = styled(Box)`
  border-radius: 50%;
  background-color: ${({ status, theme }) =>
    status === 'Unknown'
      ? theme.colors.grey[300]
      : status === 'Running'
      ? theme.colors.success
      : theme.colors.error.light};
  margin-right: 4px;
  width: 8px;
  height: 8px;
`;
