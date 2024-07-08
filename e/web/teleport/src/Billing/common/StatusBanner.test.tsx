import React from 'react';

import { screen, userEvent } from 'design/utils/testing';

import { StatusBannerProps } from 'e-teleport/Billing/types';
import { renderWithElementsAndContext } from 'e-teleport/Billing/StripeLoader/testhelper/renderWithElementsAndContext';
import { StatusBanner } from 'e-teleport/Billing/common/StatusBanner';
import { StripeSubscriptionStatus } from 'e-teleport/Billing/StripeLoader/types';

jest.mock('teleport/useStickyClusterId', () =>
  jest.fn(() => ({ clusterId: 'cluster-name', isLeafCluster: false }))
);

describe('statusBanner', () => {
  let props: StatusBannerProps;
  const defaultProductName = 'SuperSuper';

  beforeEach(() => {
    props = {
      productName: defaultProductName,
      stripeTrialEnd: 0,
      stripeSubscriptionStatus: StripeSubscriptionStatus.INCOMPLETE,
      stripeMissingPaymentMethod: false,
      stripeCurrentPeriodEnd: 0,
      stripeSubscriptionCanceled: false,
    };
  });

  test('does not render if canceled', () => {
    props.stripeSubscriptionStatus = StripeSubscriptionStatus.CANCELED;
    const { container } = renderWithElementsAndContext(
      <StatusBanner {...props} />
    );

    expect(container).toBeEmptyDOMElement();
  });

  test('opens delete dialog on click', async () => {
    props.stripeSubscriptionStatus = StripeSubscriptionStatus.ACTIVE;
    renderWithElementsAndContext(<StatusBanner {...props} />);

    expect(
      screen.queryByText('Close Teleport Account')
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: 'Cancel Plan' })
    ).toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', { name: 'Cancel Plan' }));
    await screen.findByText('Close Teleport Account');
  });

  describe('title/description is set based on status', () => {
    // eslint-disable-next-line jest/require-hook
    [
      {
        status: StripeSubscriptionStatus.TRIALING,
        title: `${defaultProductName} Trial is Activated`,
        description: `Your trial will expire on Jan 01, 1970. To maintain access to your Teleport cluster, upgrade to the Teleport ${defaultProductName} Plan.`,
        actionButton: false,
        upgraded: false,
      },
      {
        status: StripeSubscriptionStatus.TRIALING,
        title: `${defaultProductName} Plan is Activated`,
        description: `Your trial will expire on Jan 01, 1970.`,
        actionButton: true,
        upgraded: true,
      },
      {
        status: StripeSubscriptionStatus.ACTIVE,
        title: `${defaultProductName} Plan is Activated`,
        actionButton: true,
        upgraded: true,
      },
      {
        status: StripeSubscriptionStatus.PAST_DUE,
        title: `There is an issue with your ${defaultProductName} Plan`,
        description: 'Please check your payment details.',
        actionButton: false,
        upgraded: true,
      },
      {
        status: StripeSubscriptionStatus.UNPAID,
        title: `There is an issue with your ${defaultProductName} Plan`,
        description: 'Please check your payment details.',
        actionButton: false,
        upgraded: true,
      },
      {
        status: StripeSubscriptionStatus.INCOMPLETE,
        title: `You're on the ${defaultProductName} Plan`,
        actionButton: false,
        upgraded: true,
      },
      {
        status: StripeSubscriptionStatus.INCOMPLETE_EXPIRED,
        title: `You're on the ${defaultProductName} Plan`,
        actionButton: false,
        upgraded: true,
      },
    ].forEach(spec => {
      test(`renders ${spec.status} info`, () => {
        props.stripeSubscriptionStatus = spec.status;
        props.stripeMissingPaymentMethod = !spec.upgraded;
        props.productName = 'SuperSuper';
        renderWithElementsAndContext(<StatusBanner {...props} />);

        expect(screen.getByText(spec.title)).toBeInTheDocument();
        if (spec.description) {
          // eslint-disable-next-line jest/no-conditional-expect
          expect(screen.getByText(spec.description)).toBeInTheDocument();
        }
        if (spec.actionButton) {
          // eslint-disable-next-line jest/no-conditional-expect
          expect(
            screen.getByRole('button', { name: 'Cancel Plan' })
          ).toBeInTheDocument();
        } else {
          // eslint-disable-next-line jest/no-conditional-expect
          expect(
            screen.queryByRole('button', { name: 'Cancel Plan' })
          ).not.toBeInTheDocument();
        }
      });
    });
  });
});
