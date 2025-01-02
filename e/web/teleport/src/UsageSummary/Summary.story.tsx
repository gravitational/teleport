import { MemoryRouter } from 'react-router';

import { SummaryPage } from 'e-teleport/UsageSummary/SummaryPage';
import { makeUsageSummary } from 'e-teleport/UsageSummary/testHelpers';
import { ContextProvider } from 'teleport';
import { createTeleportContext } from 'teleport/mocks/contexts';

export default {
  title: 'TeleportE/Billing/Enterprise Usage-Based',
};

const ctx = createTeleportContext();

export function SummaryPageViewWithUsage() {
  const props = makeUsageSummary({
    cycleStart: 1727762400,
    cycleStartFormatted: 'Oct 01, 2024',
    cycleEnd: 1730354400,
    cycleEndFormatted: 'Oct 31, 2024',
    usageUpdatedAt: 1728926640,
    usageUpdatedAtFormatted: 'Oct 14, 2024 11:24',
    mau: {
      maximum: 1000,
      cycleCount: 45,
      free: 0,
      perMau: 0,
    },
    tpr: {
      maximum: 44004,
      cycleCount: 43009,
      free: 0,
      perMau: 5,
    },
  });

  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <SummaryPage summary={props} />
      </ContextProvider>
    </MemoryRouter>
  );
}

export function CalibratingSummaryPageView() {
  const props = makeUsageSummary({
    cycleStart: new Date('2024-01-01').getTime(),
    cycleEnd: new Date('2024-01-31').getTime(),
    salesforceIdUpdatedAt: new Date('2024-01-13').getTime(),
    hasCloudAnonymizationKey: true,
    tpr: {
      cycleCount: 500,
      maximum: 1000,
      free: 0,
      perMau: 0,
    },
    mau: {
      cycleCount: 500,
      maximum: 1000,
      free: 0,
      perMau: 0,
    },
  });

  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <SummaryPage summary={props} />
      </ContextProvider>
    </MemoryRouter>
  );
}

export function EmptySummaryPageView() {
  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <SummaryPage summary={undefined} />
      </ContextProvider>
    </MemoryRouter>
  );
}
