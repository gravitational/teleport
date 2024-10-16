import React from 'react';

import { ContextProvider } from 'teleport';

import { createTeleportContext } from 'teleport/mocks/contexts';

import { MemoryRouter } from 'react-router';

import { SummaryPage } from 'e-teleport/UsageSummary/SummaryPage';
import { makeUsageSummary } from 'e-teleport/UsageSummary/testHelpers';

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
      cycleCount: 500,
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

export function EmptySummaryPageView() {
  const props = makeUsageSummary({
    cycleStart: 1727762400,
    cycleStartFormatted: 'Oct 01, 2024',
    cycleEnd: 1730354400,
    cycleEndFormatted: 'Oct 31, 2024',
    usageUpdatedAt: 0,
    usageUpdatedAtFormatted: '',
  });

  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <SummaryPage summary={props} />
      </ContextProvider>
    </MemoryRouter>
  );
}
