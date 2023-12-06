import React from 'react';

import { screen, waitFor } from 'design/utils/testing';
import { within } from '@testing-library/react';

import userEvent from '@testing-library/user-event';

import { CycleProps } from 'e-teleport/Billing/types';
import { renderWithElementsAndContext } from 'e-teleport/Billing/StripeLoader/testhelper/renderWithElementsAndContext';
import { Cycle } from 'e-teleport/Billing/Summary/Cycle';

describe('cycle', () => {
  let props: CycleProps;

  beforeEach(() => {
    props = {
      currentUsage: {
        invoiceId: 'some-invoiceId',
        status: 'some-status',
        periodEnd: 1682989332,
        periodStart: 1672989632,
        usageMau: 0,
        usageTia: 0,
        usagePr: 0,
      },
      nonBillableUsage: {
        trustedDeviceUsage: {
          devicesInUse: 0,
          devicesUsageLimit: 0,
        },
        accessRequestUsage: {
          monthlyUsed: 0,
          monthlyLimit: 5,
        },
      },
      productName: 'some-product',
      stripeMissingPaymentMethod: false,
      stripeTrialEnd: 0,
      usageUpdatedAt: 0,
    };
  });

  test('renders cycle overview', () => {
    renderWithElementsAndContext(<Cycle {...props} />);

    expect(screen.getByText(/Current Cycle:/i)).toBeInTheDocument();
    expect(
      screen.getByText(/Jan 06, 2023 - May 02, 2023/i)
    ).toBeInTheDocument();
    expect(
      screen.getByText(/Your next invoice will occur on May 02, 2023/i)
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        /Your team plan includes a limited amount of free usage. If your team exceeds the limit for a given category, your team will be charged for the extra use./i
      )
    ).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Learn More.' })).toHaveAttribute(
      'href',
      'https://goteleport.com/teleport-pricing/'
    );
    expect(
      screen.getAllByRole('link', { name: /Contact Sales/i })
    ).toHaveLength(2);
  });

  test('info icon popovers', async () => {
    props.usageUpdatedAt = 1699538455; // 2023-11-09 14:00:55
    renderWithElementsAndContext(<Cycle {...props} />);

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
    await userEvent.unhover;

    const tia = screen.getByTestId(/Teleport Identity Authorizations/i);
    const tiaIcon = within(tia).getByRole('icon');
    await userEvent.hover(tiaIcon);
    await waitFor(() => {
      expect(
        screen.getByText(
          'The authentication or authorization by Teleport of a client connection, API request, SSH session or any other activity related to a human user or service interaction.'
        )
      ).toBeVisible();
    });
    await userEvent.unhover;

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
    await userEvent.unhover;

    const mad = screen.getByTestId(/Trusted Devices/i);
    const madIcon = within(mad).getByRole('icon');
    await userEvent.hover(madIcon);
    await waitFor(() => {
      expect(
        screen.getByText(
          'Unique trusted devices enrolled in Teleport. Upgrade to enterprise plan for more than five devices.'
        )
      ).toBeVisible();
    });
    await userEvent.unhover;

    const updated = screen.getByTestId('updated-at-display');
    const updatedIcon = within(updated).getByRole('icon');
    await userEvent.hover(updatedIcon);
    await waitFor(() => {
      expect(screen.getByText('Updated every 12 hours.')).not.toBe(0);
    });
    await userEvent.unhover;
  });

  test('renders usage', () => {
    props.currentUsage.usageMau = 0;
    props.currentUsage.usageTia = 10000;
    props.currentUsage.usagePr = 200;
    props.nonBillableUsage.trustedDeviceUsage.devicesInUse = 1;
    props.nonBillableUsage.accessRequestUsage.monthlyUsed = 3;
    props.usageUpdatedAt = 0;

    renderWithElementsAndContext(<Cycle {...props} />);
    const mau = screen.getByTestId(/Active Users/i);
    expect(within(mau).getByText(/0 of 30/i)).toBeInTheDocument();
    expect(within(mau).getByText(/\(0%\)/i)).toBeInTheDocument();

    const tia = screen.getByTestId(/Teleport Identity Authorizations/i);
    expect(
      within(tia).getByText(/10000 of 50000 Included/i)
    ).toBeInTheDocument();
    expect(within(tia).getByText(/\(20%\)/i)).toBeInTheDocument();

    const pr = screen.getByTestId(/Teleport Protected Resources/i);
    expect(within(pr).getByText(/200 of 50/i)).toBeInTheDocument();
    expect(within(pr).getByText(/\(400%\)/i)).toBeInTheDocument();

    const mad = screen.getByTestId(/Trusted Devices/i);
    expect(within(mad).getByText(/1 of 5/i)).toBeInTheDocument();
    expect(within(mad).getByText(/\(20%\)/i)).toBeInTheDocument();

    const mar = screen.getByTestId(/Access Requests/i);
    expect(within(mar).getByText(/3 of 5/i)).toBeInTheDocument();
    expect(within(mar).getByText(/\(60%\)/i)).toBeInTheDocument();
  });

  test('renders usage updated at value', () => {
    props.usageUpdatedAt = 0;
    renderWithElementsAndContext(<Cycle {...props} />);
    expect(screen.getByText('Updated every 12 hours')).toBeInTheDocument();

    props.usageUpdatedAt = 1699538455; // 2023-11-09 14:00:55
    renderWithElementsAndContext(<Cycle {...props} />);
    expect(
      screen.getByText('Last updated: 2023-11-09 14:00:55')
    ).toBeInTheDocument();
  });
});
