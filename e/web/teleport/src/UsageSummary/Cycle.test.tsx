import { render, screen, waitFor } from 'design/utils/testing';
import { within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

import { Cycle, CycleProps } from 'e-teleport/UsageSummary/Cycle';
import { makeUsageSummary } from 'e-teleport/UsageSummary/testHelpers';

describe('cycle', () => {
  let props: CycleProps;

  beforeEach(() => {
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
      }),
    };
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

    render(<Cycle {...props} />);

    expect(screen.getByText(/Current Cycle:/i)).toBeInTheDocument();
    expect(
      screen.getByText(/Jan 06, 2023 - May 02, 2023/i)
    ).toBeInTheDocument();
    expect(
      screen.getByText(/Monthly usage will reset at the end of this cycle./i)
    ).toBeInTheDocument();

    const mau = screen.getByTestId(/Active Users/i);

    expect(within(mau).getByText(/0 of 0/i)).toBeInTheDocument();
    expect(within(mau).getByText(/\(0%\)/i)).toBeInTheDocument();

    const pr = screen.getByTestId(/Teleport Protected Resources/i);

    expect(within(pr).getByText(/0 of 0/i)).toBeInTheDocument();
    expect(within(pr).getByText(/\(0%\)/i)).toBeInTheDocument();
  });

  test('info icon popovers', async () => {
    props.summary.usageUpdatedAt = 1699538455; // 2023-11-09 14:00:55
    render(<Cycle {...props} />);

    const mau = screen.getByTestId(/Active Users/i);
    const mauIcon = within(mau).getByRole('icon');
    await userEvent.hover(mauIcon);
    await waitFor(() => {
      expect(
        screen.getByText(
          'Any unique human or machine user, local or SSO username or email with recorded activity during a month.'
        )
      ).toBeVisible();
    });
    await userEvent.unhover(mauIcon);

    const tpr = screen.getByTestId(/Teleport Protected Resources/i);
    const tprIcon = within(tpr).getByRole('icon');
    await userEvent.hover(tprIcon);
    await waitFor(() => {
      expect(
        screen.getByText(
          'Any unique resource such as a Kubernetes cluster, SSH server, database instance or serverless endpoint, that has registered itself with the Teleport cluster and is protected by Teleport.'
        )
      ).toBeVisible();
    });
    await userEvent.unhover(tprIcon);

    const updated = screen.getByTestId('updated-at-display');
    const updatedIcon = within(updated).getByRole('icon');
    await userEvent.hover(updatedIcon);
    await waitFor(() => {
      expect(screen.getByText('Updated every 12 hours.')).not.toBe(0);
    });
    await userEvent.unhover(updatedIcon);
  });

  test('renders usage', () => {
    props.summary.mau.cycleCount = 0;
    props.summary.tpr.cycleCount = 80;
    props.summary.usageUpdatedAt = 0;

    render(<Cycle {...props} />);
    const mau = screen.getByTestId(/Active Users/i);

    expect(within(mau).getByText(/0 of 2/i)).toBeInTheDocument();
    expect(within(mau).getByText(/\(0%\)/i)).toBeInTheDocument();

    const pr = screen.getByTestId(/Teleport Protected Resources/i);

    expect(within(pr).getByText(/80 of 20/i)).toBeInTheDocument();
    expect(within(pr).getByText(/\(400%\)/i)).toBeInTheDocument();
  });

  test('renders usage updated at with no value', () => {
    props.summary.usageUpdatedAt = 0;
    render(<Cycle {...props} />);

    expect(screen.getByText('Updated every 12 hours')).toBeInTheDocument();
  });

  test('renders usage updated at with value', () => {
    props.summary.usageUpdatedAt = 1699538455; // 2023-11-09 14:00:55
    props.summary.usageUpdatedAtFormatted = 'Nov 09, 2023';
    render(<Cycle {...props} />);

    expect(screen.getByText('Last updated: Nov 09, 2023')).toBeInTheDocument();
  });
});

describe('calibration period', () => {
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

    render(<Cycle {...props} />);

    expect(screen.getAllByText('Calibrating...')).toHaveLength(2);
    expect(
      screen.getByText(
        'A change to your account requires a calibration period in order to accurately count Active Users and Teleport Protected Resources. This should resolve itself with the start of your next billing cycle.'
      )
    ).toBeInTheDocument();
  });
});
