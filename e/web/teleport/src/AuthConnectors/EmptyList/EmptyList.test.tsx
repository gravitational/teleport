import React from 'react';
import { render, screen, fireEvent } from 'design/utils/testing';

import { createTeleportContext } from 'teleport/mocks/contexts';
import TeleportContextProvider from 'teleport/TeleportContextProvider';

import { CtaEvent, userEventService } from 'teleport/services/userEvent';

import EmptyList from './EmptyList';

describe('emptyList', () => {
  const dummyOnCreate = jest.fn();
  const dummyShowLockedFeature = false;

  const renderComponent = (props = {}) =>
    renderWithContext(
      <EmptyList
        onCreate={dummyOnCreate}
        showLockedFeature={dummyShowLockedFeature}
        {...props}
      />
    );

  afterEach(() => {
    jest.clearAllMocks();
  });

  test('renders properly', () => {
    renderComponent();
    expect(
      screen.getByText('Create Your First Auth Connector')
    ).toBeInTheDocument();
  });

  test('calls onCreate with correct AuthProviderType when connectors are clicked', () => {
    renderComponent();
    fireEvent.click(screen.getByText('GitHub Connector'));
    expect(dummyOnCreate).toHaveBeenCalledWith('github');

    fireEvent.click(screen.getByText('OIDC Connector'));
    expect(dummyOnCreate).toHaveBeenCalledWith('oidc');

    fireEvent.click(screen.getByText('SAML Connector'));
    expect(dummyOnCreate).toHaveBeenCalledWith('saml');
  });

  test('shows locked feature message when showLockedFeature is true', () => {
    renderComponent({ showLockedFeature: true });
    expect(
      screen.getByText('Unlock OIDC & SAML with Teleport Enterprise')
    ).toBeInTheDocument();
  });

  test('triggers correct event when locked feature button is clicked', () => {
    const spy = jest.spyOn(userEventService, 'captureCtaEvent');
    renderComponent({ showLockedFeature: true });
    fireEvent.click(
      screen.getByText('Unlock OIDC & SAML with Teleport Enterprise')
    );
    expect(spy).toHaveBeenCalledWith(CtaEvent.CTA_AUTH_CONNECTOR);
  });
});

const ctx = createTeleportContext();
const renderWithContext = ui =>
  render(<TeleportContextProvider ctx={ctx}>{ui}</TeleportContextProvider>);
