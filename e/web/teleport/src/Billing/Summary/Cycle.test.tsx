import React from 'react';

import { screen } from 'design/utils/testing';
import { within } from '@testing-library/react';

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
        /Your team plan includes a limited amount of free usage. If your team exceeds the limit for a given category, your team will be charged for the extra use. Cluster owners are notified if usage approaches or exceeds a limit./i
      )
    ).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Learn More' })).toHaveAttribute(
      'href',
      'https://goteleport.com/teleport-pricing/'
    );
  });

  test('renders usage', () => {
    props.currentUsage.usageMau = 0;
    props.currentUsage.usageTia = 80;
    props.currentUsage.usagePr = 200;

    renderWithElementsAndContext(<Cycle {...props} />);
    const mau = screen.getByTestId(/Active Users/i);
    expect(within(mau).getByText(/0 of 30/i)).toBeInTheDocument();
    expect(within(mau).getByText(/(0%)/i)).toBeInTheDocument();

    const tia = screen.getByTestId(/Teleport Identity Authorizations/i);
    expect(within(tia).getByText(/80 of 12000/i)).toBeInTheDocument();
    expect(within(tia).getByText(/(1%)/i)).toBeInTheDocument();

    const pr = screen.getByTestId(/Teleport Protected Resources/i);
    expect(within(pr).getByText(/200 of 50/i)).toBeInTheDocument();
    expect(within(pr).getByText(/(400%)/i)).toBeInTheDocument();
  });
});
