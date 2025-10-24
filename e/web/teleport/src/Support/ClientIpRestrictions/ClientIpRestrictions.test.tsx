import { MemoryRouter } from 'react-router';

import { act, fireEvent, render, screen, waitFor } from 'design/utils/testing';
import { InfoGuidePanelProvider } from 'shared/components/SlidingSidePanel/InfoGuide';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { ContextProvider } from 'teleport';

import { ClientIpRestrictions } from './ClientIpRestrictions';

describe('ClientIpRestrictions', () => {
  const clusterId = 'cluster-123';

  beforeEach(() => {
    jest.resetAllMocks();
  });

  test('renders fetched allowlist', async () => {
    const ctx = createTeleportContextE();
    ctx.storeUser.geClientIpRestrictionAccess = () => ({
      read: false,
      remove: true,
      list: true,
      edit: true,
      create: true,
    });
    ctx.clientIpRestrictionsService.fetchClientIpRestrictions = jest
      .fn()
      .mockResolvedValue(['10.0.0.0/8', '192.168.0.0/16']);
    ctx.clientIpRestrictionsService.saveClientIpRestrictions = jest.fn();

    renderComponent(ctx);
    await waitFor(() => {
      expect(screen.getByRole('textbox')).toHaveDisplayValue(/10.0.0.0\/8/i);
    });
    expect(screen.getByRole('textbox')).toHaveDisplayValue(/192.168.0.0\/16/i);
  });

  test('renders error if fetching fails', async () => {
    const ctx = createTeleportContextE();
    ctx.storeUser.geClientIpRestrictionAccess = () => ({
      read: false,
      remove: true,
      list: true,
      edit: true,
      create: true,
    });
    ctx.clientIpRestrictionsService.fetchClientIpRestrictions = jest
      .fn()
      .mockRejectedValue(new Error('Failed to fetch'));

    renderComponent(ctx);
    await waitFor(() => {
      expect(screen.getByText(/Failed to fetch/i)).toBeInTheDocument();
    });
  });

  test('renders empty if no list access', () => {
    const ctx = createTeleportContextE();
    ctx.storeUser.geClientIpRestrictionAccess = () => ({
      read: false,
      remove: true,
      list: false,
      edit: false,
      create: false,
    });
    const { container } = renderComponent(ctx);
    expect(container).toBeEmptyDOMElement();
  });

  test('disables edit button without edit access', async () => {
    const ctx = createTeleportContextE();
    ctx.storeUser.geClientIpRestrictionAccess = () => ({
      read: false,
      remove: true,
      list: true,
      edit: false,
      create: false,
    });
    ctx.clientIpRestrictionsService.fetchClientIpRestrictions = jest
      .fn()
      .mockResolvedValue(['10.0.0.0/8']);

    renderComponent(ctx);
    await waitFor(() => {
      expect(screen.getByText(/edit/i)).toBeDisabled();
    });
  });

  test('saving updates the allowlist and shows success', async () => {
    const ctx = createTeleportContextE();
    ctx.storeUser.geClientIpRestrictionAccess = () => ({
      read: false,
      remove: false,
      list: true,
      edit: true,
      create: true,
    });
    ctx.clientIpRestrictionsService.fetchClientIpRestrictions = jest
      .fn()
      .mockResolvedValue(['10.0.0.0/8']);
    ctx.clientIpRestrictionsService.saveClientIpRestrictions = jest
      .fn()
      .mockResolvedValue(undefined);

    renderComponent(ctx);
    await waitFor(() => screen.findByDisplayValue(/10.0.0.0\/8/i));

    // enable editing
    act(() => {
      screen.getByText(/edit/i).click();
    });

    // add a new cidr block
    fireEvent.change(screen.getByRole('textbox'), {
      target: { value: '10.0.0.0/8\n172.16.0.0/12' },
    });

    await waitFor(() => {
      expect(screen.getByRole('textbox')).toHaveDisplayValue(
        '10.0.0.0/8\n172.16.0.0/12'
      );
    });

    // click `Save`
    act(() => {
      screen.getByRole('button', { name: 'Save' }).click();
    });

    await waitFor(() => {
      expect(screen.getByText(/Allowlist updated/i)).toBeInTheDocument();
    });
    expect(
      ctx.clientIpRestrictionsService.saveClientIpRestrictions
    ).toHaveBeenCalledWith(clusterId, ['10.0.0.0/8', '172.16.0.0/12']);
  });
});

function renderComponent(ctx: any) {
  return render(
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <InfoGuidePanelProvider>
          <ClientIpRestrictions clusterId="cluster-123" />
        </InfoGuidePanelProvider>
      </ContextProvider>
    </MemoryRouter>
  );
}
