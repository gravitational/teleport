import { MemoryRouter } from 'react-router';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { ContextProvider } from 'teleport/index';

import { usageHistory } from './fixtures';
import { UsageHistory, UsageHistoryProps } from './UsageHistory';

export default {
  title: 'TeleportE/Billing/UsageHistory',
};

function render({ history, maxMau, maxTpr }: UsageHistoryProps) {
  const ctx = createTeleportContextE();

  return (
    <MemoryRouter initialEntries={['/clusters/test-cluster']}>
      <ContextProvider ctx={ctx}>
        <UsageHistory history={history} maxMau={maxMau} maxTpr={maxTpr} />
      </ContextProvider>
    </MemoryRouter>
  );
}

export function EmptyHistory() {
  return render({
    history: [],
    maxMau: 300,
    maxTpr: 50,
  });
}

export function NonEmptyHistory() {
  return render({
    history: usageHistory,
    maxMau: 300,
    maxTpr: 50,
  });
}
