import React, { Suspense } from 'react';
import { MemoryRouter } from 'react-router';
import { render, screen, userEvent } from 'design/utils/testing';
import { ContextProvider } from 'teleport';
import { IntegrationStatusCode, Plugin } from 'teleport/services/integrations';
import { userEventService } from 'teleport/services/userEvent';
import { allAccessAcl, noAccess } from 'teleport/mocks/contexts';
import cfg from 'teleport/config';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import TeleportEContext from 'e-teleport/teleportContextE';
import { pluginsService } from 'e-teleport/services/plugins';

import { IntegrationEnroll } from '../IntegrationEnroll';

describe('test PluginPick.tsx', () => {
  const originalCloudFlag = cfg.isCloud; // should be false
  const originalIsTeamFlag = cfg.isTeam; // should be false
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
    cfg.isTeam = originalIsTeamFlag;
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
    const ctx = createTeleportContextE();
    const { container } = renderIntegrationPicker(ctx);

    await screen.findByText(/no-code integrations/i);
    expect(screen.queryByTestId('plugin-checkmark')).not.toBeInTheDocument();
    expect(screen.getByTestId('tile-slack')).toHaveAttribute('href');

    // snapshot that all tiles are rendered.
    expect(container).toMatchSnapshot();
  });

  test('clicking on already enrolled slack tile does not render slack enroll view', async () => {
    jest.spyOn(pluginsService, 'fetchPlugins').mockResolvedValue(mockPlugins);

    const ctx = createTeleportContextE();
    renderIntegrationPicker(ctx);

    await screen.findByText(/no-code integrations/i);
    expect(screen.getByTestId('plugin-checkmark')).toBeInTheDocument();

    // test clicking on slack tile does not render slack enroll view.
    await userEvent.click(screen.getByTestId('tile-slack'));
    expect(
      screen.queryByRole('button', { name: /connect slack/i })
    ).not.toBeInTheDocument();
  });

  test('no plugin access disables plugin tiles', async () => {
    const ctx = createTeleportContextE({
      customAcl: { ...allAccessAcl, plugins: noAccess },
    });
    const { container } = renderIntegrationPicker(ctx);

    await screen.findByText(/no-code integrations/i);

    // snapshot the disabled plugin access state.
    expect(container).toMatchSnapshot();

    // eslint-disable-next-line jest-dom/prefer-enabled-disabled
    expect(screen.getByTestId('tile-slack')).toHaveAttribute('disabled');
    expect(screen.getByTestId('tile-slack')).not.toHaveAttribute('href');

    // test an integration tile is not disabled by clicking on it to render guide
    await userEvent.click(screen.getByTestId('tile-aws-oidc'));
    await screen.findByText(/set up your aws account/i);
  });

  test('no integration access disables integration tiles', async () => {
    const ctx = createTeleportContextE({
      customAcl: { ...allAccessAcl, integrations: { ...noAccess, use: false } },
    });
    const { container } = renderIntegrationPicker(ctx);

    await screen.findByText(/no-code integrations/i);

    // snapshot the disabled integration access state.
    expect(container).toMatchSnapshot();

    // eslint-disable-next-line jest-dom/prefer-enabled-disabled
    expect(screen.getByTestId('tile-aws-oidc')).toHaveAttribute('disabled');
    expect(screen.getByTestId('tile-aws-oidc')).not.toHaveAttribute('href');

    // test a plugin tile is not disabled by clicking on it to render guide
    await userEvent.click(screen.getByTestId('tile-slack'));
    await screen.findByRole('button', { name: /connect slack/i });
  });

  test('disableForTeam disables jamf plugin tile in team plan', async () => {
    cfg.isTeam = true;
    const ctx = createTeleportContextE();
    renderIntegrationPicker(ctx);
    await screen.findByText(/no-code integrations/i);

    // eslint-disable-next-line jest-dom/prefer-enabled-disabled
    expect(screen.getByTestId('tile-jamf')).toHaveAttribute('disabled');
    expect(screen.getByTestId('tile-jamf')).not.toHaveAttribute('href');
  });

  test('show jamf plugin tiles in cloud plan', async () => {
    cfg.isTeam = false;
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
