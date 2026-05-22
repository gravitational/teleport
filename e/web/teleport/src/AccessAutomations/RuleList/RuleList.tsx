import { type JSX } from 'react';
import styled from 'styled-components';

import { Box, Flex, Label, MenuItem } from 'design';
import Table, { Cell } from 'design/DataTable';
import { IconTooltip } from 'design/Tooltip';
import { MenuButton } from 'shared/components/MenuAction';
import { SearchPanel } from 'shared/components/Search';
import { capitalizeFirstLetter } from 'shared/utils/text';

import {
  AccessMonitoringRule,
  AccessMonitoringRuleType,
  AccessMonitoringRuleWithYaml,
} from 'e-teleport/services/accessmonitoringrule/types';
import { SeversidePagination } from 'teleport/components/hooks/useServersidePagination';
import {
  Plugin,
  PluginDatadogSpec,
  PluginEmailSpec,
  PluginKind,
  PluginMattermostSpec,
  PluginMsTeamsSpec,
  PluginSlackSpec,
} from 'teleport/services/integrations';
import { Access } from 'teleport/services/user';

import { TableRow, TableRowFallback, TableRowRule } from './types';

export function RuleList({
  onEdit,
  onSearchChange,
  onViewTerraform,
  viewingRule,
  search,
  serversidePagination,
  plugins,
  rulesAcl,
}: {
  onEdit(rule: AccessMonitoringRuleWithYaml): void;
  onSearchChange(search: string): void;
  onViewTerraform(rule: AccessMonitoringRuleWithYaml): void;
  viewingRule: AccessMonitoringRuleWithYaml;
  search: string;
  serversidePagination: SeversidePagination<AccessMonitoringRuleWithYaml>;
  plugins: Plugin[];
  rulesAcl: Access;
}) {
  const canView = rulesAcl.list && rulesAcl.read;
  const canEdit = rulesAcl.edit;

  return (
    <Table
      data={[
        ...ruleRows(serversidePagination.fetchedData.agents),
        ...(serversidePagination.fetchNext === null
          ? fallbackRows(plugins)
          : []),
      ]}
      fetching={{
        fetchStatus: serversidePagination.fetchStatus,
        onFetchNext: serversidePagination.fetchNext,
        onFetchPrev: serversidePagination.fetchPrev,
      }}
      serversideProps={{
        sort: undefined,
        setSort: () => undefined,
        serversideSearchPanel: (
          <SearchPanel
            updateSearch={onSearchChange}
            updateQuery={null}
            hideAdvancedSearch={true}
            filter={{ search }}
            disableSearch={serversidePagination.attempt.status === 'processing'}
          />
        ),
      }}
      columns={[
        {
          key: 'name',
          headerText: 'Name',
          render: ({ name, plugin }) => {
            return <StyledCell $plugin={!!plugin}>{name}</StyledCell>;
          },
        },
        {
          key: 'types',
          headerText: 'Type',
          render: ({ types }) => {
            return renderLabelCell(types);
          },
        },
        {
          key: 'integration',
          headerText: 'Integration',
          render: ({ integration, plugin }) => {
            return <StyledCell $plugin={!!plugin}>{integration}</StyledCell>;
          },
        },
        {
          key: 'recipients',
          headerText: 'Recipients',
          render: ({ recipients }) => {
            return renderLabelCell(recipients);
          },
        },
        {
          altKey: 'view-btn',
          render: (row: TableRow) => {
            const isViewing = row.name === viewingRule?.object?.metadata?.name;
            return 'item' in row
              ? renderActionCell(
                  isViewing,
                  canView,
                  canEdit,
                  () => onEdit(row.item),
                  () => onViewTerraform(row.item)
                )
              : renderInfoCell(row.plugin);
          },
        },
      ]}
      emptyText="No Access Automations Found"
      isSearchable
      row={{
        getKey: row =>
          'item' in row
            ? `rule:${row.name}`
            : `plugin:${row.plugin.kind}:${row.plugin.name}`,
      }}
    />
  );
}

// ruleRows converts the list of rules into table rows.
function ruleRows(rules: AccessMonitoringRuleWithYaml[] = []): TableRowRule[] {
  const getIntegrations = (rule: AccessMonitoringRule): string[] => {
    const types = [
      rule.spec.automatic_review && AccessMonitoringRuleType.Review,
      rule.spec.notification && AccessMonitoringRuleType.Notification,
    ].filter(Boolean);
    return types;
  };

  return rules.map(rule => ({
    name: rule.object.metadata.name,
    types: getIntegrations(rule.object),
    integration: rule.object.spec.notification?.name,
    recipients: rule.object.spec.notification?.recipients,
    item: rule,
  }));
}

// fallbackRows converts the list of plugins into table rows.
function fallbackRows(plugins: Plugin[] = []): TableRowFallback[] {
  const getFallbackRecipients = (plugin: Plugin): string[] => {
    switch (plugin.kind) {
      case 'mattermost':
        const { channel, reportToEmail, team } =
          plugin.spec as PluginMattermostSpec;
        return [channel && team && `${team}/${channel}`, reportToEmail].filter(
          Boolean
        );
      case 'slack':
        return [(plugin.spec as PluginSlackSpec).fallbackChannel];
      case 'datadog':
        return [(plugin.spec as PluginDatadogSpec).fallbackRecipient];
      case 'msteams':
        return [(plugin.spec as PluginMsTeamsSpec).defaultRecipient];
      case 'email':
        return [(plugin.spec as PluginEmailSpec).fallbackRecipient];
      default:
        return ['unknown'];
    }
  };

  return plugins.map(plugin => ({
    name: `Fallback ${capitalizeFirstLetter(plugin.kind)} Rule`,
    types: [AccessMonitoringRuleType.Notification],
    integration: plugin.name,
    recipients: getFallbackRecipients(plugin),
    plugin,
  }));
}

function renderActionCell(
  viewing: boolean,
  canView: boolean,
  canEdit: boolean,
  onEdit: () => void,
  onViewTerraform: () => void
): JSX.Element {
  return (
    <Cell align="right" style={{ whiteSpace: 'nowrap' }}>
      <Flex alignItems="center" justifyContent="right" width="60px">
        <MenuButton>
          {(viewing || canView) && (
            <MenuItem onClick={onEdit}>
              {viewing ? 'Hide' : canEdit ? 'Edit' : 'View Details'}
            </MenuItem>
          )}
          {canView && (
            <MenuItem onClick={onViewTerraform}>View Terraform</MenuItem>
          )}
        </MenuButton>
      </Flex>
    </Cell>
  );
}

function renderInfoCell(plugin: Plugin): JSX.Element {
  const integrationName = (kind: PluginKind): string => {
    switch (kind) {
      case 'msteams':
        return 'Microsoft Teams';
      default:
        return capitalizeFirstLetter(kind);
    }
  };

  const recipientTypes = (kind: PluginKind): string[] | [string, string] => {
    switch (kind) {
      case 'slack':
        return ['channel'];
      case 'mattermost':
        return ['email', 'team/channel'];
      case 'msteams':
        return ['email', 'channel'];
      case 'datadog':
        return ['email', 'team'];
      case 'email':
        return ['email'];
      default:
        return ['recipient'];
    }
  };

  return (
    <Cell align="right" style={{ whiteSpace: 'nowrap' }}>
      <Flex alignItems="center" width="60px">
        <Box css={{ margin: '0 auto' }}>
          <IconTooltip>
            Fallback notification rule for {integrationName(plugin?.kind)}. The
            default {recipientTypes(plugin?.kind).join(' or ')} will receive all
            notifications about access requests. This rule will <i>not</i> be
            used for access requests that are matched by custom rules.
          </IconTooltip>
        </Box>
      </Flex>
    </Cell>
  );
}

function renderLabelCell(labels: string[] = []): JSX.Element {
  return (
    <StyledCell>
      <Flex flexWrap="wrap" gap={1}>
        {labels.map((label, index) => (
          <Label key={`${label}${index}`} kind="secondary">
            {label}
          </Label>
        ))}
      </Flex>
    </StyledCell>
  );
}

const StyledCell = styled(Cell)<{ $plugin?: boolean }>`
  color: ${p => (p.$plugin ? p.theme.colors.text.muted : 'inherit')} !important;
`;
