import { MemoryRouter } from 'react-router';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { ContextProvider } from 'teleport/index';

import { usageHistory } from './fixtures';
import { UsageHistory, UsageHistoryProps } from './UsageHistory';

export default {
  title: 'TeleportE/Billing/UsageHistory',
};

function ExampleHistory({
  history,
  hasCloudAnonymizationKey,
  salesforceIdUpdatedAt,
}: UsageHistoryProps) {
  const ctx = createTeleportContextE();

  return (
    <MemoryRouter initialEntries={['/clusters/test-cluster']}>
      <ContextProvider ctx={ctx}>
        <UsageHistory
          history={history}
          hasCloudAnonymizationKey={hasCloudAnonymizationKey}
          salesforceIdUpdatedAt={salesforceIdUpdatedAt}
        />
      </ContextProvider>
    </MemoryRouter>
  );
}

export function EmptyHistory() {
  return (
    <ExampleHistory
      history={[]}
      hasCloudAnonymizationKey={false}
      salesforceIdUpdatedAt={0}
    />
  );
}

export function NonEmptyHistory() {
  return (
    <ExampleHistory
      history={usageHistory}
      hasCloudAnonymizationKey={true}
      salesforceIdUpdatedAt={usageHistory[1].cycleStart + 1}
    />
  );
}
