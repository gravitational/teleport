import { fireEvent, render, screen } from 'design/utils/testing';

import { createTeleportContext } from 'teleport/mocks/contexts';
import TeleportContextProvider from 'teleport/TeleportContextProvider';

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
      screen.getByText('Select a service provider below')
    ).toBeInTheDocument();
  });

  test('calls onCreate with correct AuthProviderType when connectors are clicked', () => {
    renderComponent();
    fireEvent.click(screen.getByText('GitHub'));
    expect(dummyOnCreate).toHaveBeenCalledWith('github');

    fireEvent.click(screen.getByText('OIDC'));
    expect(dummyOnCreate).toHaveBeenCalledWith('oidc');

    fireEvent.click(screen.getByText('SAML'));
    expect(dummyOnCreate).toHaveBeenCalledWith('saml');
  });

  test('shows locked feature message when showLockedFeature is true', () => {
    renderComponent({ showLockedFeature: true });
    expect(
      screen.getByText('Unlock OIDC & SAML with Teleport Enterprise')
    ).toBeInTheDocument();
  });
});

const ctx = createTeleportContext();
ctx.isEnterprise = true;
const renderWithContext = ui =>
  render(<TeleportContextProvider ctx={ctx}>{ui}</TeleportContextProvider>);
