import React from 'react';

import { screen } from 'design/utils/testing';

import { CancelBannerProps } from 'e-teleport/Billing/types';
import { renderWithElementsAndContext } from 'e-teleport/Billing/StripeLoader/testhelper/renderWithElementsAndContext';
import { CancelBanner } from 'e-teleport/Billing/common/CancelBanner';

describe('cancelButton', () => {
  let props: CancelBannerProps;

  beforeEach(() => {
    props = {
      productName: 'some-productName',
      stripeTrialEnd: 0,
      stripeMissingPaymentMethod: true,
    };
  });

  test('renders cancel banner with cancel CTA if upgraded', () => {
    props.stripeMissingPaymentMethod = false;
    props.productName = 'SuperSuper';
    renderWithElementsAndContext(<CancelBanner {...props} />);

    expect(
      screen.getByText('SuperSuper Plan is Activated')
    ).toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: 'Cancel Plan' })
    ).toBeInTheDocument();
  });

  test('renders cancel banner with cancel info if on trial', () => {
    props.stripeMissingPaymentMethod = true;
    props.productName = 'SuperSuper';
    renderWithElementsAndContext(<CancelBanner {...props} />);

    expect(
      screen.getByText('SuperSuper Trial is Activated')
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        /Your trial will expire on Jan 01, 1970. To maintain access to your Teleport cluster, upgrade to the Teleport Team Plan./i
      )
    ).toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: 'Cancel Plan' })
    ).not.toBeInTheDocument();
  });
});
