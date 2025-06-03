import { MemoryRouter } from 'react-router';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { ContextProvider } from 'teleport/index';

import { usageHistory } from './fixtures';
import { UsageHistory, UsageHistoryProps } from './UsageHistory';

export default {
  title: 'TeleportE/Billing/UsageHistory',
};

function ExampleHistory({
  cloud,
  history,
  hasCloudAnonymizationKey,
  salesforceIdUpdatedAt,
  hasIdentityGovernance,
  hasIdentitySecurity,
}: UsageHistoryProps) {
  const ctx = createTeleportContextE();

  return (
    <MemoryRouter initialEntries={['/clusters/test-cluster']}>
      <ContextProvider ctx={ctx}>
        <UsageHistory
          cloud={cloud}
          history={history}
          hasCloudAnonymizationKey={hasCloudAnonymizationKey}
          salesforceIdUpdatedAt={salesforceIdUpdatedAt}
          hasIdentityGovernance={hasIdentityGovernance}
          hasIdentitySecurity={hasIdentitySecurity}
        />
      </ContextProvider>
    </MemoryRouter>
  );
}

export function EmptyHistory() {
  return (
    <ExampleHistory
      cloud
      history={[]}
      hasCloudAnonymizationKey={false}
      salesforceIdUpdatedAt={0}
      hasIdentityGovernance={true}
      hasIdentitySecurity={true}
    />
  );
}

export function NonEmptyHistory() {
  return (
    <ExampleHistory
      cloud
      history={usageHistory}
      hasCloudAnonymizationKey={true}
      salesforceIdUpdatedAt={usageHistory[1].cycleStart + 1}
      hasIdentityGovernance={true}
      hasIdentitySecurity={true}
    />
  );
}

export function WithoutSecurityAndIdentity() {
  return (
    <ExampleHistory
      cloud
      history={usageHistory}
      hasCloudAnonymizationKey={true}
      salesforceIdUpdatedAt={usageHistory[1].cycleStart + 1}
      hasIdentityGovernance={false}
      hasIdentitySecurity={false}
    />
  );
}
