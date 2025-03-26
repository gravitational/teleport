import { within } from '@testing-library/react';
import type { ReactNode } from 'react';

import { render, screen } from 'design/utils/testing';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { Cycle, CycleProps } from 'e-teleport/UsageSummary/Cycle';
import { makeUsageSummary } from 'e-teleport/UsageSummary/testHelpers';
import cfg from 'teleport/config';
import { ContextProvider } from 'teleport/index';

const defaultEntitlements = cfg.entitlements;

function renderWithContext(component: ReactNode) {
  const ctx = createTeleportContextE();

  return render(<ContextProvider ctx={ctx}>{component}</ContextProvider>);
}

describe('cycle', () => {
  let props: CycleProps;

  beforeEach(() => {
    cfg.entitlements.Identity.enabled = true;
    cfg.entitlements.Policy.enabled = true;
    props = {
      summary: makeUsageSummary({
        cycleEnd: 1682989332,
        cycleEndFormatted: 'May 02, 2023',
        cycleStart: 1672989632,
        cycleStartFormatted: 'Jan 06, 2023',
        usageUpdatedAt: 0,
        mau: {
          maximum: 2,
          free: 2,
          perMau: 0,
          cycleCount: 1,
        },
        tpr: {
          maximum: 20,
          free: 20,
          perMau: 0,
          cycleCount: 0,
        },
        mwi: {
          maximum: 2,
          free: 2,
          perMau: 0,
          cycleCount: 1,
        },
        igmau: {
          maximum: 4,
          free: 4,
          perMau: 0,
          cycleCount: 3,
        },
      }),
    };
  });

  afterEach(() => {
    cfg.entitlements = defaultEntitlements;
  });

  test('renders cycle overview, handles 0, no calibration period', () => {
    props.summary.tpr = {
      maximum: 0,
      free: 0,
      perMau: 0,
      cycleCount: 0,
    };
    props.summary.mau = {
      maximum: 0,
      free: 0,
      perMau: 0,
      cycleCount: 0,
    };
    props.summary.mwi = {
      maximum: 0,
      free: 0,
      perMau: 0,
      cycleCount: 0,
    };
    props.summary.igmau = {
      maximum: 0,
      free: 0,
      perMau: 0,
      cycleCount: 0,
    };

    renderWithContext(<Cycle {...props} />);

    expect(screen.getByText(/Current Billing Cycle:/i)).toBeInTheDocument();
    expect(
      screen.getByText(/Jan 06, 2023 - May 02, 2023/i)
    ).toBeInTheDocument();
    expect(
      screen.getByText(/Monthly usage will reset at the end of this cycle/i)
    ).toBeInTheDocument();

    const mau = screen.getAllByTestId(/Active Users/i)[0];

    expect(within(mau).getByText(/0 of 0/i)).toBeInTheDocument();
    expect(within(mau).getByText(/\(0%\)/i)).toBeInTheDocument();

    const pr = screen.getAllByTestId(/Teleport Protected Resources/i)[0];

    expect(within(pr).getByText(/0 of 0/i)).toBeInTheDocument();
    expect(within(pr).getByText(/\(0%\)/i)).toBeInTheDocument();

    const mwi = screen.getAllByTestId(/MWI/i)[0];

    expect(within(mwi).getByText(/0 of 0/i)).toBeInTheDocument();
    expect(within(mwi).getByText(/\(0%\)/i)).toBeInTheDocument();

    const igmau = screen.getAllByTestId(/Active Users/i)[1];

    expect(within(igmau).getByText(/0 of 0/i)).toBeInTheDocument();
    expect(within(igmau).getByText(/\(0%\)/i)).toBeInTheDocument();
  });

  test('renders usage', () => {
    props.summary.mau.cycleCount = 0;
    props.summary.tpr.cycleCount = 80;
    props.summary.mwi.cycleCount = 800;
    props.summary.igmau.cycleCount = 4;
    props.summary.usageUpdatedAt = 0;

    renderWithContext(<Cycle {...props} />);
    const mau = screen.getAllByTestId(/Monthly Active Users \(MAU\)/i)[0];

    expect(within(mau).getByText(/0 of 2/i)).toBeInTheDocument();
    expect(within(mau).getByText(/\(0%\)/i)).toBeInTheDocument();

    const pr = screen.getAllByTestId(/Teleport Protected Resources/i)[0];

    expect(within(pr).getByText(/80 of 20/i)).toBeInTheDocument();
    expect(within(pr).getByText(/\(400%\)/i)).toBeInTheDocument();

    const mwi = screen.getByTestId(/MWI/i);

    expect(within(mwi).getByText(/800 of 2/i)).toBeInTheDocument();
    expect(within(mwi).getByText(/\(40000%\)/i)).toBeInTheDocument();

    const igmau = screen.getAllByTestId(/Monthly Active Users \(MAU\)/i)[1];

    expect(within(igmau).getByText(/4 of 4/i)).toBeInTheDocument();
    expect(within(igmau).getByText(/\(100%\)/i)).toBeInTheDocument();

    const ispr = screen.getAllByTestId(/Teleport Protected Resources/i)[1];

    expect(within(ispr).getByText(/80 of 20/i)).toBeInTheDocument();
    expect(within(ispr).getByText(/\(400%\)/i)).toBeInTheDocument();
  });

  test('renders usage updated at with no value', () => {
    props.summary.usageUpdatedAt = 0;
    renderWithContext(<Cycle {...props} />);

    expect(screen.getByText('Updated every 12 hours')).toBeInTheDocument();
    expect(screen.queryByText(/Last updated/)).not.toBeInTheDocument();
  });

  test('renders usage updated at with value', () => {
    props.summary.usageUpdatedAt = 1699538455; // 2023-11-09 14:00:55
    props.summary.usageUpdatedAtFormatted = 'Nov 09, 2023';
    renderWithContext(<Cycle {...props} />);

    expect(screen.getByText('Last updated: Nov 09, 2023')).toBeInTheDocument();
    expect(screen.getByText(/Updated every 12 hours/)).toBeInTheDocument();
  });

  test('show IGS CTA if Identity is disabled', () => {
    cfg.entitlements.Identity.enabled = false;
    renderWithContext(<Cycle {...props} />);

    const ctaButton = screen.getByRole('link', { name: /Upgrade Now/i });
    expect(ctaButton).toBeInTheDocument();
    expect(ctaButton).toHaveProperty(
      'href',
      'https://goteleport.com/r/upgrade-policy?e_4.4.0-dev&utm_campaign=CTA_UNSPECIFIED'
    );
  });

  test('show IS CTA if Policy is disabled', () => {
    cfg.entitlements.Policy.enabled = false;
    renderWithContext(<Cycle {...props} />);

    const ctaButton = screen.getByRole('link', { name: /Upgrade Now/i });
    expect(ctaButton).toBeInTheDocument();
    expect(ctaButton).toHaveProperty(
      'href',
      'https://goteleport.com/r/upgrade-igs?e_4.4.0-dev&utm_campaign=CTA_UNSPECIFIED'
    );
  });
});

describe('calibration period', () => {
  beforeAll(() => {
    cfg.entitlements.Identity.enabled = true;
    cfg.entitlements.Policy.enabled = true;
  });

  afterAll(() => {
    cfg.entitlements = defaultEntitlements;
  });

  test.each`
    desc                        | start                               | end                                 | updated
    ${'updated on cycle start'} | ${new Date('2024-01-01').getTime()} | ${new Date('2024-01-31').getTime()} | ${new Date('2024-01-01').getTime()}
    ${'updated on cycle end'}   | ${new Date('2024-01-01').getTime()} | ${new Date('2024-01-31').getTime()} | ${new Date('2024-01-31').getTime()}
    ${'updated mid cycle'}      | ${new Date('2024-01-01').getTime()} | ${new Date('2024-01-31').getTime()} | ${new Date('2024-01-15').getTime()}
  `('renders for $desc', ({ start, end, updated }) => {
    const props = {
      summary: makeUsageSummary({
        cycleEnd: end,
        cycleStart: start,
        salesforceIdUpdatedAt: updated,
        hasCloudAnonymizationKey: true,
      }),
    };

    renderWithContext(<Cycle {...props} />);

    expect(screen.getAllByText('Calibrating')).toHaveLength(5);
    expect(screen.getByText('Calibration In Progress')).toBeInTheDocument();
  });
});
