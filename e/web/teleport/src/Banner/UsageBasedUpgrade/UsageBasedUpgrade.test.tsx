import React from 'react';
import { fireEvent, screen } from 'design/utils/testing';

import { add, getUnixTime } from 'date-fns';

import { displayShortDate } from 'shared/services/loc/loc';

import { BillingInformation } from 'e-teleport/services/cloud';
import { UsageBasedUpgrade } from 'e-teleport/Banner/UsageBasedUpgrade/UsageBasedUpgrade';
import { UsageBasedUpgradeProps } from 'e-teleport/Banner/UsageBasedUpgrade/types';
import { renderWithElementsAndContext } from 'e-teleport/Billing/StripeLoader/testhelper/renderWithElementsAndContext';
import { StripeSubscriptionStatus } from 'e-teleport/Billing/StripeLoader/types';

describe('usageBasedUpgrade', () => {
  let props: UsageBasedUpgradeProps;

  beforeEach(() => {
    props = {
      billingInfo: makeBillingInfo(),
      reload: () => null,
    };
  });

  it('does not render if stripe subscription is past trial period', () => {
    props.billingInfo.usageBasedBilling = true;
    props.billingInfo.stripeTrial = false;

    renderWithElementsAndContext(<UsageBasedUpgrade {...props} />);

    expect(screen.queryByTestId('upgrade-banner')).not.toBeInTheDocument();
  });

  it('does not render if usage based billing is false', () => {
    props.billingInfo.usageBasedBilling = false;

    renderWithElementsAndContext(<UsageBasedUpgrade {...props} />);

    expect(screen.queryByTestId('upgrade-banner')).not.toBeInTheDocument();
  });

  test('if canceled, renders cancel banner', () => {
    props.billingInfo.usageBasedBilling = true;
    props.billingInfo.stripeSubscriptionStatus =
      StripeSubscriptionStatus.CANCELED;
    renderWithElementsAndContext(<UsageBasedUpgrade {...props} />);

    expect(
      screen.getByText(/Your account has been canceled/i)
    ).toBeInTheDocument();
  });

  it('opens modal on click', () => {
    props.billingInfo.usageBasedBilling = true;
    props.billingInfo.stripeTrial = true;
    props.billingInfo.stripeMissingPaymentMethod = true;

    renderWithElementsAndContext(<UsageBasedUpgrade {...props} />);

    expect(screen.getByTestId('upgrade-banner')).toBeInTheDocument();
    expect(screen.queryByTestId('dialog')).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /Upgrade/i }));
    expect(screen.getByTestId('Modal')).toBeInTheDocument();
  });

  it('sets display message for trial period, adds upgrade CTA button', () => {
    props.billingInfo = makeBillingInfo({
      usageBasedBilling: true,
      stripeTrial: true,
      stripeMissingPaymentMethod: true,
      productName: 'Test',
    });

    renderWithElementsAndContext(<UsageBasedUpgrade {...props} />);

    expect(screen.getByTestId('upgrade-banner')).toBeInTheDocument();
    expect(screen.getByTestId('message')).toHaveTextContent(
      /To maintain access Upgrade Test Trial to Test Plan./
    );
    expect(screen.getByRole('button', { name: 'Upgrade' })).toBeInTheDocument();
  });

  it('sets display message for trial period after adding payment with billing cycle date', () => {
    const date = new Date('2023-04-12T11:00:00.00Z');
    const unix = getUnixTime(date);

    props.billingInfo = makeBillingInfo({
      usageBasedBilling: true,
      stripeTrial: true,
      productName: 'Test',
      stripeTrialEnd: unix,
    });

    renderWithElementsAndContext(<UsageBasedUpgrade {...props} />);

    expect(screen.getByTestId('upgrade-banner')).toBeInTheDocument();
    expect(screen.getByTestId('message')).toHaveTextContent(
      /Your first monthly billing cycle will start on Apr 13, 2023/
    );
    expect(
      screen.queryByRole('button', { name: 'Upgrade' })
    ).not.toBeInTheDocument();
  });

  describe('calculates days remaining in trial', () => {
    beforeEach(() => {
      // enable banner display
      props.billingInfo.usageBasedBilling = true;
      props.billingInfo.stripeTrial = true;
    });

    it('on last day of trial', () => {
      const trialEnd = new Date();
      const dayAfter = displayShortDate(add(trialEnd, { days: 1 }));
      props.billingInfo.stripeTrialEnd = getUnixTime(trialEnd);

      renderWithElementsAndContext(<UsageBasedUpgrade {...props} />);

      expect(screen.getByTestId('upgrade-banner')).toBeInTheDocument();
      expect(screen.getByTestId('message')).toHaveTextContent(
        `Your trial expires today. Your first monthly billing cycle will start on ${dayAfter}.`
      );
    });

    for (let i = 2; i < 15; i++) {
      it(`for ${i} day(s) from now`, () => {
        const trialEnd = add(new Date(), { days: i });
        const dayAfter = displayShortDate(add(trialEnd, { days: 1 }));
        props.billingInfo.stripeTrialEnd = getUnixTime(trialEnd);

        renderWithElementsAndContext(<UsageBasedUpgrade {...props} />);

        expect(screen.getByTestId('upgrade-banner')).toBeInTheDocument();
        expect(screen.getByTestId('message')).toHaveTextContent(
          `Your trial expires in ${i} days. Your first monthly billing cycle will start on ${dayAfter}.`
        );
      });
    }
  });
});

const makeBillingInfo = (
  overrides: Partial<BillingInformation> = {}
): BillingInformation => {
  return Object.assign(
    {
      defaultPaymentMethodId: '',
      cardsList: [],
      bankAccountsList: [],
      stripePublicKey: '',
      productName: '',
      trial: false,
      selfEnrolled: false,
      upsellAlert: false,
      stripeCustomerId: '',
      stripeTrial: false,
      stripeTrialEnd: 0,
      usageBasedBilling: false,
      stripeMissingPaymentMethod: false,
      stripeSubscriptionStatus: StripeSubscriptionStatus.ACTIVE,
    },
    overrides
  );
};
