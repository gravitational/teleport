import { PointerEventsCheckLevel } from '@testing-library/user-event';
import { Suspense } from 'react';
import { MemoryRouter } from 'react-router';

import { render, screen, userEvent } from 'design/utils/testing';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { pluginsService } from 'e-teleport/services/plugins';
import TeleportEContext from 'e-teleport/teleportContextE';
import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';
import { allAccessAcl, noAccess } from 'teleport/mocks/contexts';
import { IntegrationStatusCode, Plugin } from 'teleport/services/integrations';
import { userEventService } from 'teleport/services/userEvent';

import { IntegrationEnroll } from '../IntegrationEnroll';

describe('test PluginPick.tsx', () => {
  const originalCloudFlag = cfg.isCloud; // should be false
  const originalMdmEntitlement = cfg.entitlements.MobileDeviceManagement;
  beforeEach(() => {
    cfg.isCloud = true;
    jest
      .spyOn(pluginsService, 'fetchAvailableTypes')
      .mockResolvedValue(['slack', 'jamf']);
    jest.spyOn(pluginsService, 'fetchPlugins').mockResolvedValue([]);
    jest
      .spyOn(userEventService, 'captureIntegrationEnrollEvent')
      .mockImplementation();
  });

  afterEach(() => {
    cfg.isCloud = originalCloudFlag;
    cfg.entitlements.MobileDeviceManagement = originalMdmEntitlement;
    jest.clearAllMocks();
  });

  test('full access and non-cloud, does not render slack', async () => {
    cfg.isCloud = false;
    const ctx = createTeleportContextE();
    renderIntegrationPicker(ctx);

    await screen.findByText(/no-code integrations/i);
    expect(screen.queryByText(/slack/i)).not.toBeInTheDocument();
    expect(screen.getByText(/oidc/i)).toBeInTheDocument();
  });

  test('full access and slack available to enroll', async () => {
    cfg.externalAuditStorage = true;
    cfg.entitlements.MobileDeviceManagement = { enabled: true, limit: 0 };
    const ctx = createTeleportContextE();
    renderIntegrationPicker(ctx);

    await screen.findByText(/no-code integrations/i);
    expect(screen.queryByTestId('plugin-checkmark')).not.toBeInTheDocument();
    expect(screen.getByTestId('tile-slack')).toHaveAttribute('href');
  });

  test('clicking on already enrolled slack tile does not render slack enroll view', async () => {
    jest.spyOn(pluginsService, 'fetchPlugins').mockResolvedValue(mockPlugins);

    const ctx = createTeleportContextE();
    renderIntegrationPicker(ctx);

    await screen.findByText(/no-code integrations/i);
    expect(screen.getByTestId('plugin-checkmark')).toBeInTheDocument();

    // test clicking on slack tile has no pointer events.
    await userEvent.click(screen.getByTestId('tile-slack'), {
      pointerEventsCheck: PointerEventsCheckLevel.Never,
    });
  });

  test('no plugin access disables plugin tiles', async () => {
    cfg.externalAuditStorage = true;
    cfg.entitlements.MobileDeviceManagement = { enabled: true, limit: 0 };
    const ctx = createTeleportContextE({
      customAcl: { ...allAccessAcl, plugins: noAccess },
    });
    renderIntegrationPicker(ctx);
    await screen.findByText(/no-code integrations/i);

    // eslint-disable-next-line jest-dom/prefer-enabled-disabled
    expect(screen.getByTestId('tile-slack')).toHaveAttribute('disabled');
    expect(screen.getByTestId('tile-slack')).not.toHaveAttribute('href');

    // test an integration tile is not disabled by clicking on it to render guide
    await userEvent.click(screen.getByTestId('tile-aws-oidc'));
    await screen.findByText(/set up your aws account/i);
  });

  test('no integration access disables integration tiles', async () => {
    cfg.entitlements.MobileDeviceManagement = { enabled: true, limit: 0 };
    const ctx = createTeleportContextE({
      customAcl: { ...allAccessAcl, integrations: { ...noAccess, use: false } },
    });
    renderIntegrationPicker(ctx);
    await screen.findByText(/no-code integrations/i);

    // eslint-disable-next-line jest-dom/prefer-enabled-disabled
    expect(screen.getByTestId('tile-aws-oidc')).toHaveAttribute('disabled');
    expect(screen.getByTestId('tile-aws-oidc')).not.toHaveAttribute('href');

    // test a plugin tile is not disabled by clicking on it to render guide
    await userEvent.click(screen.getByTestId('tile-slack'));
    await screen.findByRole('button', { name: /connect slack/i });
  });

  test('disables jamf plugin tile in plans without MDM', async () => {
    cfg.entitlements.MobileDeviceManagement = { enabled: false, limit: 0 };
    const ctx = createTeleportContextE();
    renderIntegrationPicker(ctx);
    await screen.findByText(/no-code integrations/i);

    // eslint-disable-next-line jest-dom/prefer-enabled-disabled
    expect(screen.getByTestId('tile-jamf')).toHaveAttribute('disabled');
    expect(screen.getByTestId('tile-jamf')).not.toHaveAttribute('href');
  });

  test('show jamf plugin tiles in cloud plan', async () => {
    cfg.entitlements.MobileDeviceManagement = { enabled: true, limit: 0 };
    const ctx = createTeleportContextE();
    renderIntegrationPicker(ctx);
    await screen.findByText(/no-code integrations/i);

    expect(screen.getByTestId('tile-jamf')).toHaveAttribute('href');
  });
});

function renderIntegrationPicker(ctx: TeleportEContext) {
  return render(
    <MemoryRouter
      initialEntries={[{ pathname: cfg.getIntegrationEnrollRoute() }]}
    >
      <Suspense fallback={null}>
        <ContextProvider ctx={ctx}>
          <IntegrationEnroll />
        </ContextProvider>
      </Suspense>
    </MemoryRouter>
  );
}

const mockPlugins: Plugin[] = [
  {
    resourceType: 'plugin',
    name: 'plugin-name',
    details: 'some detail',
    spec: {},
    kind: 'slack',
    statusCode: IntegrationStatusCode.Running,
  },
];
