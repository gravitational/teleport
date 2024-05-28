import React from 'react';
import styled from 'styled-components';
import { Alert, ButtonBorder, Flex, Text, ButtonText, Box } from 'design';
import Table, { Cell } from 'design/DataTable';
import { useInfiniteScroll } from 'shared/hooks';
import { Attempt } from 'shared/hooks/useAttemptNext';
import {
  Plugin,
  PluginMattermostSpec,
  PluginOpsgenieSpec,
  PluginSlackSpec,
} from 'teleport/services/integrations';
import { capitalizeFirstLetter } from 'shared/utils/text';
import { ToolTipInfo } from 'shared/components/ToolTip';

import {
  AccessMonitoringRule,
  AccessMonitoringRuleWithYaml,
} from 'e-teleport/services/accessmonitoringrule/types';

type RowBase = {
  name: string;
  integration: string;
  recipients: string;
};

type TableRowRule = RowBase & {
  item?: AccessMonitoringRuleWithYaml;
  plugin?: never;
};

type TableRowFallback = RowBase & {
  plugin?: Plugin;
  item?: never;
};

export function NotificationRoutingRuleList({
  attempt,
  fetch,
  rules,
  viewingRule,
  toggleViewingRule,
  plugins,
}: {
  attempt: Attempt;
  fetch(options?: { force?: boolean }): Promise<void>;
  rules: AccessMonitoringRuleWithYaml[];
  viewingRule: AccessMonitoringRule;
  toggleViewingRule(r: AccessMonitoringRuleWithYaml): void;
  plugins: Plugin[];
}) {
  const { setTrigger } = useInfiniteScroll({
    fetch: fetch,
  });

  function retryAttempt() {
    fetch({ force: true });
  }

  const badRequest = attempt.statusCode === 400 || attempt.statusCode === 403;

  const pluginsForTable = makePluginsForTable(plugins);
  const rulesForTable: TableRowRule[] = rules.map(r => ({
    name: r.object.metadata.name,
    integration: r.object.spec.notification?.name,
    recipients: r.object.spec.notification?.recipients?.join(', '),
    item: r,
  }));

  return (
    <>
      {attempt.status === 'failed' && (
        <Alert kind="danger">
          <Flex alignItems="center">
            <Text>{attempt.statusText}</Text>
            {!badRequest && (
              <ButtonText onClick={retryAttempt} width="100px">
                Retry
              </ButtonText>
            )}
          </Flex>
        </Alert>
      )}
      <Table
        data={[...rulesForTable, ...pluginsForTable]}
        columns={[
          {
            key: 'name',
            headerText: 'Name',
            render: ({ name, plugin }) => (
              <StyledCell $plugin={!!plugin}>{name}</StyledCell>
            ),
          },
          {
            key: 'integration',
            headerText: 'Integration',
            render: ({ integration, plugin }) => (
              <StyledCell $plugin={!!plugin}>{integration}</StyledCell>
            ),
          },
          {
            key: 'recipients',
            headerText: 'Recipients',
            render: ({ recipients, plugin }) => (
              <StyledCell $plugin={!!plugin}>{recipients}</StyledCell>
            ),
          },
          {
            altKey: 'view-btn',
            render: rule => {
              if (rule.item) {
                return renderActionCell(
                  rule.item,
                  viewingRule,
                  () => toggleViewingRule(rule.item),
                  attempt.status
                );
              } else {
                return renderInfoCell(rule.plugin);
              }
            },
          },
        ]}
        emptyText="No Notification Routing Rules Found"
        isSearchable
      />
      <div ref={setTrigger} />
    </>
  );
}

function makePluginsForTable(plugins: Plugin[]): TableRowFallback[] {
  return plugins.map(plugin => {
    const name = `Fallback ${capitalizeFirstLetter(plugin.kind)} Rule`;
    const integration = plugin.name;
    let recipients = '';

    if (!plugin.spec) {
      return {
        name,
        integration,
        recipients: 'unknown',
        plugin,
      };
    }

    switch (plugin.kind) {
      case 'slack': {
        const { fallbackChannel } = plugin.spec as PluginSlackSpec;
        recipients = fallbackChannel;
        break;
      }
      case 'mattermost': {
        const { channel, reportToEmail } = plugin.spec as PluginMattermostSpec;
        if (channel && reportToEmail) {
          recipients = `${channel} (channel), ${reportToEmail} (email)`;
        } else if (channel) {
          recipients = `${channel} (channel)`;
        } else if (reportToEmail) {
          recipients = `${reportToEmail} (email)`;
        }
        break;
      }
      case 'opsgenie': {
        const { defaultSchedules } = plugin.spec as PluginOpsgenieSpec;
        recipients = defaultSchedules?.join(', ');
        break;
      }
      default: {
        recipients = 'unknown';
      }
    }

    return {
      name,
      integration,
      recipients,
      plugin,
    };
  });
}

const renderActionCell = (
  thisRule: AccessMonitoringRuleWithYaml,
  viewingRule: AccessMonitoringRule,
  toggleViewingRule: () => void,
  attemptStatus: Attempt['status']
) => {
  const viewingThisRow =
    viewingRule && viewingRule?.metadata.name === thisRule.object.metadata.name;
  return (
    <Cell align="right" style={{ whiteSpace: 'nowrap' }}>
      <Flex alignItems="center" justifyContent="right" width="60px">
        <ButtonBorder
          size="small"
          ml={3}
          onClick={toggleViewingRule}
          disabled={attemptStatus === 'processing'}
        >
          {viewingThisRow ? 'Hide' : 'View'}
        </ButtonBorder>
      </Flex>
    </Cell>
  );
};

// "The default channel will receive all notifications about access requests.
// Request notifications will also be sent directly to assigned reviewers (if any)."
const renderInfoCell = (plugin: Plugin) => {
  if (plugin.kind === 'slack') {
    return (
      <Cell align="right" style={{ whiteSpace: 'nowrap' }}>
        <Flex alignItems="center" width="60px">
          <Box css={{ margin: '0 auto' }}>
            <ToolTipInfo>
              Fallback notification rule for{' '}
              {capitalizeFirstLetter(plugin.kind)}. The default channel will
              receive all notifications about access requests. This rule will{' '}
              <b>not</b> be used for access requests that are matched by custom
              rules.
            </ToolTipInfo>
          </Box>
        </Flex>
      </Cell>
    );
  }
  return null;
};

const StyledCell = styled(Cell)`
  color: ${p => (p.$plugin ? p.theme.colors.text.muted : 'inherit')} !important;
`;
