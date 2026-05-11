import { PointerEventsCheckLevel } from '@testing-library/user-event';
import { Suspense } from 'react';
import { MemoryRouter } from 'react-router';

import { render, screen, userEvent, waitFor } from 'design/utils/testing';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { pluginsService } from 'e-teleport/services/plugins';
import TeleportEContext from 'e-teleport/teleportContextE';
import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';
import * as Main from 'teleport/Main/Main';
import { allAccessAcl, noAccess } from 'teleport/mocks/contexts';
import { IntegrationStatusCode, Plugin } from 'teleport/services/integrations';
import { userEventService } from 'teleport/services/userEvent';

import { IntegrationEnroll } from '../IntegrationEnroll';

describe('test PluginPick.tsx', () => {
  const originalCloudFlag = cfg.isCloud; // should be false
  const originalExternalAuditStorageEntitlement =
    cfg.entitlements.ExternalAuditStorage;
  beforeEach(() => {
    cfg.isCloud = true;
    jest
      .spyOn(pluginsService, 'fetchAvailableTypes')
      .mockResolvedValue(['slack', 'jamf']);
    jest.spyOn(pluginsService, 'fetchPlugins').mockResolvedValue([]);
    jest
      .spyOn(userEventService, 'captureIntegrationEnrollEvent')
      .mockImplementation();
    jest.spyOn(Main, 'useNoMinWidth').mockReturnValue();
  });

  afterEach(() => {
    cfg.isCloud = originalCloudFlag;
    cfg.entitlements.ExternalAuditStorage =
      originalExternalAuditStorageEntitlement;
    jest.clearAllMocks();
  });

  test('full access and non-cloud, does not render slack', async () => {
    cfg.isCloud = false;
    const ctx = createTeleportContextE();
    renderIntegrationPicker(ctx);

    await screen.findByText(/Integration Type/i);
    expect(screen.queryByText(/slack/i)).not.toBeInTheDocument();
    expect(screen.getByText(/AWS OIDC Identity Provider/i)).toBeInTheDocument();
  });

  test('full access and slack available to enroll', async () => {
    cfg.entitlements.ExternalAuditStorage = { enabled: true, limit: 0 };
    const ctx = createTeleportContextE();
    renderIntegrationPicker(ctx);

    await screen.findByText(/Integration Type/i);
    expect(screen.queryByTestId('plugin-checkmark')).not.toBeInTheDocument();
    expect(screen.getByTestId('tile-slack')).toHaveAttribute('href');
    await waitFor(() => {
      expect(
        screen.queryByText(/You do not have permission to create Integrations/i)
      ).not.toBeInTheDocument();
    });
  });

  test('clicking on already enrolled slack tile does not render slack enroll view', async () => {
    jest.spyOn(pluginsService, 'fetchPlugins').mockResolvedValue(mockPlugins);

    const ctx = createTeleportContextE();
    renderIntegrationPicker(ctx);

    await screen.findByText(/Integration Type/i);
    expect(screen.getByTestId('integration-checkmark')).toBeInTheDocument();

    // test clicking on slack tile has no pointer events.
    await userEvent.click(screen.getByTestId('tile-slack'), {
      pointerEventsCheck: PointerEventsCheckLevel.Never,
    });
  });

  test('no plugin or integration access shows permission banner', async () => {
    const ctx = createTeleportContextE({
      customAcl: {
        ...allAccessAcl,
        plugins: noAccess,
        integrations: { ...noAccess, use: false },
      },
    });
    renderIntegrationPicker(ctx);
    expect(
      screen.getByText(/You do not have permission to create Integrations/i)
    ).toBeInTheDocument();
  });

  test('no plugin access disables plugin tiles', async () => {
    cfg.entitlements.ExternalAuditStorage = { enabled: true, limit: 0 };
    const ctx = createTeleportContextE({
      customAcl: { ...allAccessAcl, plugins: noAccess },
    });
    renderIntegrationPicker(ctx);
    await screen.findByText(/Integration Type/i);

    // eslint-disable-next-line jest-dom/prefer-enabled-disabled
    expect(screen.getByTestId('tile-slack')).toHaveAttribute('disabled');
    expect(screen.getByTestId('tile-slack')).not.toHaveAttribute('href');
    await waitFor(() => {
      expect(
        screen.queryByText(/You do not have permission to create Integrations/i)
      ).not.toBeInTheDocument();
    });

    // test an integration tile is not disabled by clicking on it to render guide
    await userEvent.click(screen.getByTestId('tile-aws-oidc'));
    await screen.findByText(/set up your aws account/i);
  });

  test('no integration access disables integration tiles', async () => {
    const ctx = createTeleportContextE({
      customAcl: { ...allAccessAcl, integrations: { ...noAccess, use: false } },
    });
    renderIntegrationPicker(ctx);
    await screen.findByText(/Integration Type/i);

    // eslint-disable-next-line jest-dom/prefer-enabled-disabled
    expect(screen.getByTestId('tile-aws-oidc')).toHaveAttribute('disabled');
    expect(screen.getByTestId('tile-aws-oidc')).not.toHaveAttribute('href');
    await waitFor(() => {
      expect(
        screen.queryByText(/You do not have permission to create Integrations/i)
      ).not.toBeInTheDocument();
    });

    // test a plugin tile is not disabled by clicking on it to render guide
    await userEvent.click(screen.getByTestId('tile-slack'));
    await screen.findByRole('button', { name: /connect slack/i });
  });

  test('show jamf plugin tiles without MDM entitlement', async () => {
    cfg.entitlements.MobileDeviceManagement = { enabled: false, limit: 0 };
    const ctx = createTeleportContextE();
    renderIntegrationPicker(ctx);
    await screen.findByText(/Integration Type/i);

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
