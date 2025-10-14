import { StoryObj } from '@storybook/react-vite';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { http, HttpResponse } from 'msw';
import { MemoryRouter } from 'react-router';

import { Info } from 'design/Alert';
import { CollapsibleInfoSection as CollapsibleInfoSectionComponent } from 'design/CollapsibleInfoSection';
import { InfoGuidePanelProvider } from 'shared/components/SlidingSidePanel/InfoGuide';

import cfg from 'e-teleport/config';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { Summary } from 'e-teleport/UsageSummary/Summary';
import {
  makeGetUsageResponse,
  makeUsageCycle,
} from 'e-teleport/UsageSummary/testHelpers';
import { ContextProvider } from 'teleport/index';
import { ContentMinWidth } from 'teleport/Main/Main';

export default {
  title: 'TeleportE/Usage',
  decorators: [
    Story => {
      const queryClient = new QueryClient();
      const ctx = createTeleportContextE() as any;
      return (
        <MemoryRouter>
          <ContextProvider ctx={ctx}>
            <InfoGuidePanelProvider>
              <QueryClientProvider client={queryClient}>
                <ContentMinWidth>
                  <CollapsibleInfoSectionComponent openLabel="Devs Instructions">
                    <Info
                      kind="info"
                      details="Aggregate cases require multiple calls to the same endpoint with a different payload. For the story, the first and second call are mocked with `once:true`; meaning you can toggle views but after the first set, the calls will end up failing."
                    >
                      Aggregate Cases
                    </Info>
                  </CollapsibleInfoSectionComponent>
                  <Story />
                </ContentMinWidth>
              </QueryClientProvider>
            </InfoGuidePanelProvider>
          </ContextProvider>
        </MemoryRouter>
      );
    },
  ],
};

const defaultResponse = makeGetUsageResponse({
  alerts: [],
  usageHistory: [
    makeUsageCycle({
      usage: {
        igmau: 34,
        ztamau: 55,
        tpr: 301,
        mwi: 29,
      },
      usageLimits: {
        igmau: 30,
        ztamau: 65,
        tpr: 1000,
        mwi: 50,
      },
    }),
    makeUsageCycle({
      usage: {
        igmau: 54,
        ztamau: 78,
        tpr: 299,
        mwi: 31,
      },
      usageLimits: {
        igmau: 30,
        ztamau: 65,
        tpr: 1000,
        mwi: 50,
      },
    }),
  ],
  aggregateCount: 3,
});

export const Aggregate = {
  parameters: {
    msw: [
      http.post(
        cfg.api.billingSummaryPath,
        () => {
          return HttpResponse.json(defaultResponse);
        },
        { once: true }
      ),
      http.post(
        cfg.api.billingSummaryPath,
        () => {
          return HttpResponse.json({
            ...defaultResponse,
            usageHistory: [
              makeUsageCycle({
                usage: {
                  igmau: 23,
                  ztamau: 40,
                  tpr: 107,
                  mwi: 30,
                },
                usageLimits: {
                  igmau: 30,
                  ztamau: 65,
                  tpr: 1000,
                  mwi: 50,
                },
              }),
            ],
          });
        },
        { once: true }
      ),
    ],
  },
  render: () => {
    return <Summary />;
  },
} satisfies StoryObj<typeof Summary>;

export const WithAPIAlerts = {
  parameters: {
    msw: [
      http.post(cfg.api.billingSummaryPath, () => {
        return HttpResponse.json({
          ...defaultResponse,
          alerts: [
            {
              name: 'About Aggregate Totals',
              details:
                'Teleport cannot match self-hosted users with cloud users. When usage from both is combined, the same person may be counted more than once, making totals appear higher than the actual number of users. For counts within a single cluster, view cluster-level data.',
              kind: 'info',
              dismissible: false,
            },
            {
              name: 'Alert-001',
              details: 'There is a usage alert',
              kind: 'info',
              dismissible: true,
            },
            {
              name: 'Alert-002',
              details: 'There is a usage alert',
              kind: 'danger',
              dismissible: false,
            },
            {
              name: 'Alert-003',
              details: 'There is a usage alert',
              kind: 'neutral',
              dismissible: true,
            },
            {
              name: 'Alert-004',
              details: 'There is a good usage alert',
              kind: 'success',
              dismissible: true,
            },
            {
              name: undefined,
              details: 'There is a usage alert without a name',
              kind: 'info',
              dismissible: true,
            },
            {
              name: 'Alert-005 without details',
              details: undefined,
              kind: 'info',
              dismissible: false,
            },
            {
              name: 'Alert-006',
              details: 'There is a usage alert without kind',
              kind: undefined,
              dismissible: true,
            },
            {
              name: 'Alert-007',
              details: 'There is a good usage alert without dismissable',
              kind: 'info',
              dismissible: undefined,
            },
            {
              name: 'Alert-008',
              details: 'invalid kind',
              kind: 'type-not-matched',
              dismissible: true,
            },
            // missing name & details; does not render
            {
              name: undefined,
              details: undefined,
              kind: 'danger',
              dismissible: true,
            },
          ],
          usageHistory: [
            makeUsageCycle({
              usage: {
                igmau: 23,
                ztamau: 40,
                tpr: 107,
                mwi: 30,
              },
              usageLimits: {
                igmau: 30,
                ztamau: 65,
                tpr: 1000,
                mwi: 50,
              },
            }),
          ],
        });
      }),
    ],
  },
  render: () => {
    return <Summary />;
  },
} satisfies StoryObj<typeof Summary>;

export const AggregateWithCta = {
  parameters: {
    msw: [
      http.post(
        cfg.api.billingSummaryPath,
        () => {
          return HttpResponse.json({
            ...defaultResponse,
            missingEntitlements: ['Identity', 'Policy'],
          });
        },
        { once: true }
      ),
      http.post(
        cfg.api.billingSummaryPath,
        () => {
          return HttpResponse.json({
            ...defaultResponse,
            usageHistory: [
              makeUsageCycle({
                usage: {
                  igmau: 23,
                  ztamau: 40,
                  tpr: 107,
                  mwi: 30,
                },
                usageLimits: {
                  igmau: 30,
                  ztamau: 65,
                  tpr: 1000,
                  mwi: 50,
                },
              }),
            ],
          });
        },
        { once: true }
      ),
    ],
  },
  render: () => {
    return <Summary />;
  },
} satisfies StoryObj<typeof Summary>;

export const AggregateCalibrating = {
  parameters: {
    msw: [
      http.post(
        cfg.api.billingSummaryPath,
        () => {
          return HttpResponse.json({
            ...defaultResponse,
            usageHistory: [
              makeUsageCycle({ calibratingAccounts: 1 }),
              ...defaultResponse.usageHistory,
            ],
          });
        },
        { once: true }
      ),
      http.post(
        cfg.api.billingSummaryPath,
        () => {
          return HttpResponse.json({
            ...defaultResponse,
            usageHistory: [
              makeUsageCycle({
                usage: {
                  igmau: 23,
                  ztamau: 40,
                  tpr: 107,
                  mwi: 30,
                },
                usageLimits: {
                  igmau: 30,
                  ztamau: 65,
                  tpr: 1000,
                  mwi: 50,
                },
              }),
            ],
          });
        },
        { once: true }
      ),
    ],
  },
  render: () => {
    return <Summary />;
  },
} satisfies StoryObj<typeof Summary>;

export const Tenant = {
  parameters: {
    msw: [
      http.post(cfg.api.billingSummaryPath, () => {
        return HttpResponse.json({ ...defaultResponse, aggregateCount: 1 });
      }),
    ],
  },
  render: () => {
    return <Summary />;
  },
} satisfies StoryObj<typeof Summary>;

export const TenantWithCTA = {
  parameters: {
    msw: [
      http.post(cfg.api.billingSummaryPath, () => {
        return HttpResponse.json({
          ...defaultResponse,
          aggregateCount: 1,
          missingEntitlements: ['Identity', 'Policy'],
        });
      }),
    ],
  },
  render: () => {
    return <Summary />;
  },
} satisfies StoryObj<typeof Summary>;

export const TenantCalibrating = {
  parameters: {
    msw: [
      http.post(cfg.api.billingSummaryPath, () => {
        return HttpResponse.json({
          ...defaultResponse,
          usageHistory: [
            makeUsageCycle({ calibratingAccounts: 1 }),
            ...defaultResponse.usageHistory,
          ],
          aggregateCount: 1,
        });
      }),
    ],
  },
  render: () => {
    return <Summary />;
  },
} satisfies StoryObj<typeof Summary>;

export const Empty = {
  parameters: {
    msw: [
      http.post(cfg.api.billingSummaryPath, () => {
        return HttpResponse.json({});
      }),
    ],
  },
  render: () => {
    return <Summary />;
  },
} satisfies StoryObj<typeof Summary>;

export const Error = {
  parameters: {
    msw: [
      http.post(cfg.api.billingSummaryPath, () => {
        return HttpResponse.json(
          {
            error: { message: 'Error loading usage.' },
          },
          { status: 400 }
        );
      }),
    ],
  },
  render: () => {
    return <Summary />;
  },
} satisfies StoryObj<typeof Summary>;
