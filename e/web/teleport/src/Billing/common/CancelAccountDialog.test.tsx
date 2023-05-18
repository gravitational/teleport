import React from 'react';
import { fireEvent, screen, theme } from 'design/utils/testing';

import { CancelDialogProps } from 'e-teleport/Billing/types';
import { renderWithElementsAndContext } from 'e-teleport/Billing/StripeLoader/testhelper/renderWithElementsAndContext';
import { CancelAccountDialog } from 'e-teleport/Billing/common/CancelAccountDialog';

describe('cancelAccountDialog', () => {
  let props: CancelDialogProps;

  beforeEach(() => {
    props = {
      open: true,
      setOpen: jest.fn(),
      productName: 'SuperProTeamEnterprise',
      stripeCurrentPeriodEnd: 1684165960,
      tenant: 'some-name',
    };
  });

  test('renders', () => {
    renderWithElementsAndContext(<CancelAccountDialog {...props} />);

    expect(screen.getByText('Close Teleport Account')).toBeInTheDocument();

    screen.getByText((content, node) => {
      const hasText = node =>
        node.textContent ===
        'You are about to cancel your Teleport SuperProTeamEnterprise Plan. If you continue, at the end of your billing cycle on May 15, 2023:';
      const nodeHasText = hasText(node);
      const childrenDontHaveText = Array.from(node.children).every(
        child => !hasText(child)
      );
      return nodeHasText && childrenDontHaveText;
    });

    expect(
      screen.getByText('Access to Teleport will be cut off.')
    ).toBeInTheDocument();
    expect(
      screen.getByText('Teleport will issue a final invoice.')
    ).toBeInTheDocument();
    expect(
      screen.getByText('Your entire account will be deleted 7–14 days later.')
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        'Once deleted, your Teleport account cannot be recovered.'
      )
    ).toBeInTheDocument();

    screen.getByText((content, node) => {
      const hasText = node =>
        node.textContent ===
        "Please confirm your cluster's subdomain, some-name below:";
      const nodeHasText = hasText(node);
      const childrenDontHaveText = Array.from(node.children).every(
        child => !hasText(child)
      );

      return nodeHasText && childrenDontHaveText;
    });

    screen.getByText((content, node) => {
      const hasText = node =>
        node.textContent ===
        'Please confirm your choice by typing close my account below:';
      const nodeHasText = hasText(node);
      const childrenDontHaveText = Array.from(node.children).every(
        child => !hasText(child)
      );

      return nodeHasText && childrenDontHaveText;
    });

    expect(
      screen.getByRole('button', { name: 'Cancel Plan and Close Account' })
    ).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Cancel' })).toBeInTheDocument();
  });

  it('show error state', () => {
    renderWithElementsAndContext(<CancelAccountDialog {...props} />);

    const labelConfirmed = screen.getByTestId('label-confirmed');
    const inputConfirmed = screen.getByTestId('input-confirmed');
    const labelSubdomain = screen.getByTestId('label-subdomain');
    const inputSubdomain = screen.getByTestId('input-subdomain');

    // no error when inputs are empty
    expect(labelConfirmed).not.toHaveStyle(`color: ${theme.colors.error.main}`);
    expect(inputConfirmed).not.toHaveStyle(
      `border: 2px solid ${theme.colors.error.main}`
    );
    expect(labelSubdomain).not.toHaveStyle(`color: ${theme.colors.error.main}`);
    expect(inputSubdomain).not.toHaveStyle(
      `border: 2px solid ${theme.colors.error.main}`
    );

    // add incorrect text to inputs
    fireEvent.change(inputConfirmed, { target: { value: 'something' } });
    fireEvent.change(inputSubdomain, { target: { value: 'something' } });

    expect(labelConfirmed).toHaveStyle(`color: ${theme.colors.error.main}`);
    expect(inputConfirmed).toHaveStyle(
      `border: 2px solid ${theme.colors.error.main}`
    );
    expect(labelSubdomain).toHaveStyle(`color: ${theme.colors.error.main}`);
    expect(inputSubdomain).toHaveStyle(
      `border: 2px solid ${theme.colors.error.main}`
    );
  });

  test('does not render zero date', () => {
    props.stripeCurrentPeriodEnd = 0;

    renderWithElementsAndContext(<CancelAccountDialog {...props} />);

    expect(screen.getByText('Close Teleport Account')).toBeInTheDocument();
    screen.getByText((content, node) => {
      const hasText = node =>
        node.textContent ===
        'You are about to cancel your Teleport SuperProTeamEnterprise Plan. If you continue, at the end of your billing cycle:';
      const nodeHasText = hasText(node);
      const childrenDontHaveText = Array.from(node.children).every(
        child => !hasText(child)
      );
      return nodeHasText && childrenDontHaveText;
    });
  });
});
