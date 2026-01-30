import { within } from '@testing-library/react';

import { render, screen } from 'design/utils/testing';

import {
  makeGetUsageResponse,
  makeUsage,
  makeUsageCycle,
  makeUsageLimits,
} from 'e-teleport/UsageSummary/testHelpers';

import { Cycle, CycleProps } from './Cycle';

describe('cycle', () => {
  let props: CycleProps;

  beforeEach(() => {
    props = {
      aggregate: false,
      customer: makeGetUsageResponse(),
      usageResponse: makeGetUsageResponse({
        usageHistory: [
          makeUsageCycle({
            end: 1682989332,
            endFormatted: 'May 02, 2023',
            start: 1672989632,
            startFormatted: 'Jan 06, 2023',
            usage: makeUsage({
              ztamau: 1,
              tpr: 0,
              mwi: 1,
              igmau: 3,
            }),
          }),
        ],
        missingEntitlements: [],
        usageUpdatedAt: 0,
      }),
    };
  });

  test('renders cycle overview, handles 0, no calibration period', () => {
    props.usageResponse.usageHistory[0].usage = makeUsage({
      tpr: 0,
      ztamau: 0,
      mwi: 0,
      igmau: 0,
    });

    render(<Cycle {...props} />);

    expect(screen.getByText(/Current Cycle:/i)).toBeInTheDocument();
    expect(
      screen.getByText(/Jan 06, 2023 - May 02, 2023/i)
    ).toBeInTheDocument();
    expect(
      screen.getByText(/Monthly usage will reset at the end of this cycle/i)
    ).toBeInTheDocument();

    const zeroTrustSection = screen.getByTestId(/Zero Trust Access/);
    const mau = within(zeroTrustSection).getByTestId(
      /Monthly Active Users \(MAU\)/
    );

    expect(within(mau).getByText(/0 of 0/i)).toBeInTheDocument();
    expect(within(mau).getByText(/\(0%\)/i)).toBeInTheDocument();

    const pr = within(zeroTrustSection).getByTestId(
      /Teleport Protected Resources/
    );

    expect(within(pr).getByText(/0 of 0/i)).toBeInTheDocument();
    expect(within(pr).getByText(/\(0%\)/i)).toBeInTheDocument();

    const mwi = screen.getByTestId(/Machine and Workload Identities/);

    expect(within(mwi).getByText(/0 of 0/i)).toBeInTheDocument();
    expect(within(mwi).getByText(/\(0%\)/i)).toBeInTheDocument();

    const igmau = screen.getByTestId(/Identity Governance/);

    expect(within(igmau).getByText(/0 of 0/i)).toBeInTheDocument();
    expect(within(igmau).getByText(/\(0%\)/i)).toBeInTheDocument();
  });

  test('renders usage', () => {
    props.usageResponse.usageHistory[0].usage = makeUsage({
      tpr: 80,
      ztamau: 0,
      mwi: 800,
      igmau: 4,
    });
    props.usageResponse.usageHistory[0].usageLimits = makeUsageLimits({
      tpr: 20,
      ztamau: 2,
      mwi: 2,
      igmau: 4,
    });
    props.usageResponse.usageUpdatedAt = 0;

    render(<Cycle {...props} />);
    const zeroTrustSection = screen.getByTestId(/Zero Trust Access/i);
    const mau = within(zeroTrustSection).getByTestId(
      /Monthly Active Users \(MAU\)/
    );

    expect(within(mau).getByText(/0 of 2/i)).toBeInTheDocument();
    expect(within(mau).getByText(/\(0%\)/i)).toBeInTheDocument();

    const pr = within(zeroTrustSection).getByTestId(
      /Teleport Protected Resources/
    );

    expect(within(pr).getByText(/80 of 20/i)).toBeInTheDocument();
    expect(within(pr).getByText(/\(400%\)/i)).toBeInTheDocument();

    const mwiSection = screen.getByTestId(/Machine and Workload Identities/);
    const mwi = within(mwiSection).getByTestId(/Machine & Workload Identities/);

    expect(within(mwi).getByText(/800 of 2/i)).toBeInTheDocument();
    expect(within(mwi).getByText(/\(40000%\)/i)).toBeInTheDocument();

    const igmau = screen.getByTestId(/Identity Governance/);

    expect(within(igmau).getByText(/4 of 4/i)).toBeInTheDocument();
    expect(within(igmau).getByText(/\(100%\)/i)).toBeInTheDocument();

    const identitySecuritySection = screen.getByTestId(/Identity Security/i);
    const ispr = within(identitySecuritySection).getByTestId(
      /Teleport Protected Resources/
    );

    expect(within(ispr).getByText(/80 of 20/i)).toBeInTheDocument();
    expect(within(ispr).getByText(/\(400%\)/i)).toBeInTheDocument();
  });

  test('renders usage updated at with value', () => {
    props.usageResponse.usageUpdatedAt = 1699538455000; // 2023-11-09 14:00:55
    render(<Cycle {...props} />);

    expect(
      screen.getByText('Last Sync: 2 years ago | Updated every 12 hours')
    ).toBeInTheDocument();
  });

  test('show IGS CTA if Identity is disabled', () => {
    props.usageResponse.missingEntitlements = ['Identity'];
    render(<Cycle {...props} />);

    const sub = screen.getByTestId('identity_governance-cta');
    expect(
      within(sub).getByText("This feature isn't part of your current plan.")
    ).toBeInTheDocument();
  });

  test('show IS CTA if Policy is disabled', () => {
    props.usageResponse.missingEntitlements = ['Policy'];
    render(<Cycle {...props} />);

    const sub = screen.getByTestId('identity_security-cta');
    expect(
      within(sub).getByText("This feature isn't part of your current plan.")
    ).toBeInTheDocument();
  });
});

test('calibration', () => {
  const props = {
    usageResponse: makeGetUsageResponse({
      usageHistory: [
        makeUsageCycle({
          calibratingAccounts: 1,
        }),
      ],
    }),
    customer: makeGetUsageResponse(),
    aggregate: false,
  };

  render(<Cycle {...props} />);

  expect(screen.getAllByText('Calibrating')).toHaveLength(5);
});

test('no calibration', () => {
  const props = {
    customer: makeGetUsageResponse(),
    aggregate: false,
    usageResponse: makeGetUsageResponse({
      usageHistory: [
        makeUsageCycle({
          calibratingAccounts: 0,
        }),
      ],
    }),
  };

  render(<Cycle {...props} />);

  expect(screen.queryByText('Calibrating')).not.toBeInTheDocument();
  expect(screen.queryByText('Calibration In Progress')).not.toBeInTheDocument();
});
