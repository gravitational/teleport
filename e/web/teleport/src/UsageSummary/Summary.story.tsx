import { MemoryRouter } from 'react-router';

import { InfoGuidePanelProvider } from 'shared/components/SlidingSidePanel/InfoGuide';

import { Summary } from 'e-teleport/UsageSummary/Summary';
import {
  makeGetBillingSummaryInformationResponse,
  makeUsageSummary,
} from 'e-teleport/UsageSummary/testHelpers';
import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';
import { ContentMinWidth } from 'teleport/Main/Main';
import { createTeleportContext } from 'teleport/mocks/contexts';

export default {
  title: 'TeleportE/Usage',
};

export function LoadedWithCta() {
  cfg.entitlements.Identity = { enabled: false, limit: 0 };
  cfg.entitlements.Policy = { enabled: false, limit: 0 };
  const ctx = createTeleportContext() as any;
  ctx.cloudService = {
    fetchBillingSummaryInformation: () =>
      Promise.resolve(
        makeGetBillingSummaryInformationResponse({
          usageSummary: makeUsageSummary({
            cycleStart: new Date('Oct 01, 2024').getTime(),
            cycleStartFormatted: 'Oct 01, 2024',
            cycleEnd: new Date('Oct 31, 2024').getTime(),
            cycleEndFormatted: 'Oct 31, 2024',
            usageUpdatedAt: new Date('Oct 14, 2024 11:24').getTime() / 1000,
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
            mwi: {
              maximum: 500,
              cycleCount: 49,
              free: 0,
              perMau: 0.5,
            },
            igmau: {
              maximum: 0,
              cycleCount: 0,
              free: 0,
              perMau: 0,
            },
            usageHistory: [
              {
                cycleStart: new Date('Oct 01, 2024').getTime(),
                cycleStartFormatted: 'Oct 01, 2024',
                cycleEnd: new Date('Oct 31, 2024').getTime(),
                cycleEndFormatted: 'Oct 31, 2024',
                mau: 45,
                tpr: 43009,
                mwi: 20,
                igmau: 0,
              },
              {
                cycleStart: new Date('Sep 01, 2024').getTime(),
                cycleStartFormatted: 'Sep 01, 2024',
                cycleEnd: new Date('Aug 31, 2024').getTime(),
                cycleEndFormatted: 'Aug 31, 2024',
                mau: 52,
                tpr: 43020,
                mwi: 33,
                igmau: 0,
              },
              {
                cycleStart: new Date('Aug 01, 2024').getTime(),
                cycleStartFormatted: 'Aug 01, 2024',
                cycleEnd: new Date('Jul 31, 2024').getTime(),
                cycleEndFormatted: 'Jul 31, 2024',
                mau: 38,
                tpr: 42120,
                mwi: 38,
                igmau: 0,
              },
            ],
          }),
        })
      ),
  };

  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <InfoGuidePanelProvider>
          <ContentMinWidth>
            <Summary />
          </ContentMinWidth>
        </InfoGuidePanelProvider>
      </ContextProvider>
    </MemoryRouter>
  );
}

export function Loaded() {
  cfg.entitlements.Identity = { enabled: true, limit: 0 };
  cfg.entitlements.Policy = { enabled: true, limit: 0 };

  const ctx = createTeleportContext() as any;
  ctx.cloudService = {
    fetchBillingSummaryInformation: () =>
      Promise.resolve(
        makeGetBillingSummaryInformationResponse({
          usageSummary: makeUsageSummary({
            cycleStart: new Date('Oct 01, 2024').getTime(),
            cycleStartFormatted: 'Oct 01, 2024',
            cycleEnd: new Date('Oct 31, 2024').getTime(),
            cycleEndFormatted: 'Oct 31, 2024',
            usageUpdatedAt: new Date('Oct 14, 2024 11:24').getTime() / 1000,
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
            mwi: {
              maximum: 500,
              cycleCount: 49,
              free: 0,
              perMau: 0.5,
            },
            igmau: {
              maximum: 20,
              cycleCount: 16,
              free: 0,
              perMau: 0,
            },
            usageHistory: [
              {
                cycleStart: new Date('Oct 01, 2024').getTime(),
                cycleStartFormatted: 'Oct 01, 2024',
                cycleEnd: new Date('Oct 31, 2024').getTime(),
                cycleEndFormatted: 'Oct 31, 2024',
                mau: 45,
                tpr: 43009,
                mwi: 20,
                igmau: 10,
              },
              {
                cycleStart: new Date('Sep 01, 2024').getTime(),
                cycleStartFormatted: 'Sep 01, 2024',
                cycleEnd: new Date('Aug 31, 2024').getTime(),
                cycleEndFormatted: 'Aug 31, 2024',
                mau: 52,
                tpr: 43020,
                mwi: 33,
                igmau: 40,
              },
              {
                cycleStart: new Date('Aug 01, 2024').getTime(),
                cycleStartFormatted: 'Aug 01, 2024',
                cycleEnd: new Date('Jul 31, 2024').getTime(),
                cycleEndFormatted: 'Jul 31, 2024',
                mau: 38,
                tpr: 42120,
                mwi: 38,
                igmau: 2,
              },
            ],
          }),
        })
      ),
  };
  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <InfoGuidePanelProvider>
          <ContentMinWidth>
            <Summary />
          </ContentMinWidth>
        </InfoGuidePanelProvider>
      </ContextProvider>
    </MemoryRouter>
  );
}

export function CalibratingSummaryView() {
  cfg.entitlements.Identity = { enabled: true, limit: 0 };
  cfg.entitlements.Policy = { enabled: true, limit: 0 };
  const ctx = createTeleportContext() as any;
  ctx.cloudService = {
    fetchBillingSummaryInformation: () =>
      Promise.resolve(
        makeGetBillingSummaryInformationResponse({
          usageSummary: makeUsageSummary({
            cloud: true,
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
            mwi: {
              cycleCount: 500,
              maximum: 1000,
              free: 0,
              perMau: 0,
            },
            igmau: {
              cycleCount: 500,
              maximum: 1000,
              free: 0,
              perMau: 0,
            },
          }),
        })
      ),
  };
  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <InfoGuidePanelProvider>
          <ContentMinWidth>
            <Summary />
          </ContentMinWidth>
        </InfoGuidePanelProvider>
      </ContextProvider>
    </MemoryRouter>
  );
}

export function EmptySummaryView() {
  const ctx = createTeleportContext() as any;
  ctx.cloudService = {
    fetchBillingSummaryInformation: () =>
      Promise.resolve(makeGetBillingSummaryInformationResponse({})),
  };

  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <InfoGuidePanelProvider>
          <ContentMinWidth>
            <Summary />
          </ContentMinWidth>
        </InfoGuidePanelProvider>
      </ContextProvider>
    </MemoryRouter>
  );
}

export function Error() {
  const ctx = createTeleportContext() as any;
  ctx.cloudService = {
    fetchBillingSummaryInformation: () =>
      Promise.reject({ message: 'error getting usage' }),
  };

  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <InfoGuidePanelProvider>
          <ContentMinWidth>
            <Summary />
          </ContentMinWidth>
        </InfoGuidePanelProvider>
      </ContextProvider>
    </MemoryRouter>
  );
}
