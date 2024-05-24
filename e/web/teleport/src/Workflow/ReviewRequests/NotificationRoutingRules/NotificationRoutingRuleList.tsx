import React from 'react';
import { Alert, ButtonBorder, Flex, Text, ButtonText } from 'design';
import Table, { Cell } from 'design/DataTable';
import { useInfiniteScroll } from 'shared/hooks';
import { Attempt } from 'shared/hooks/useAttemptNext';

import {
  AccessMonitoringRule,
  AccessMonitoringRuleWithYaml,
} from 'e-teleport/services/accessmonitoringrule/types';

export function NotificationRoutingRuleList({
  attempt,
  fetch,
  rules,
  viewingRule,
  toggleViewingRule,
}: {
  attempt: Attempt;
  fetch(options?: { force?: boolean }): Promise<void>;
  rules: AccessMonitoringRuleWithYaml[];
  viewingRule: AccessMonitoringRule;
  toggleViewingRule(r: AccessMonitoringRuleWithYaml): void;
}) {
  const { setTrigger } = useInfiniteScroll({
    fetch: fetch,
  });

  function retryAttempt() {
    fetch({ force: true });
  }

  const badRequest = attempt.statusCode === 400 || attempt.statusCode === 403;

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
        data={rules.map(r => ({
          name: r.object.metadata.name,
          integration: r.object.spec.notification?.name,
          recipients: r.object.spec.notification?.recipients?.join(', '),
          item: r,
        }))}
        columns={[
          {
            key: 'name',
            headerText: 'Name',
            render: ({ name }) => <Cell>{name}</Cell>,
          },
          {
            key: 'integration',
            headerText: 'Integration',
            render: ({ integration }) => <Cell>{integration}</Cell>,
          },
          {
            key: 'recipients',
            headerText: 'Recipients',
            render: ({ recipients }) => <Cell>{recipients}</Cell>,
          },
          {
            altKey: 'view-btn',
            render: rule =>
              renderActionCell(
                rule.item,
                viewingRule,
                () => toggleViewingRule(rule.item),
                attempt.status
              ),
          },
        ]}
        emptyText="No Notification Routing Rules Found"
        isSearchable
      />
      <div ref={setTrigger} />
    </>
  );
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
      <Flex alignItems="center" justifyContent="right" width="184px">
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
