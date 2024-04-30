import React, { useEffect, PropsWithChildren } from 'react';
import { MemoryRouter } from 'react-router';
import { rest } from 'msw';
import { initialize, mswLoader } from 'msw-storybook-addon';

import cfg from 'teleport/config';
import {
  IntegrationStatusCode,
  PluginOktaSpec,
} from 'teleport/services/integrations';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { ContextProvider } from 'teleport';
import { Info } from 'design/Alert';

import ecfg from 'e-teleport/config';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';

import { renderPluginEnroll } from '../../PluginEnroll.story';
import { PluginProvider, usePlugin } from '../usePlugin';
import { CloudHostablePlugin, pluginMap } from '../../plugins';
import { PluginEnrollSuccess } from '../PluginEnrollSuccess';

import { SetUpScim as SetUpScimComponent } from './SetUpScim';
import { ImportUserGroupsAndApps as ImportComponent } from './ImportUserGroupsAndApps/ImportUserGroupsAndApps';

initialize();

const oktaPlugin = pluginMap['okta'] as CloudHostablePlugin;

const defaultIsEnterprise = cfg.isEnterprise;
const defaultIgs = cfg.isIgsEnabled;

export default {
  title: 'TeleportE/Integrations/Enroll/Okta',
  loaders: [mswLoader],
  decorators: [
    Story => {
      useEffect(() => {
        // Clean up
        return () => {
          cfg.isEnterprise = defaultIsEnterprise;
          cfg.isIgsEnabled = defaultIgs;
        };
      }, []);
      return <Story />;
    },
  ],
};

export const EnrollOktaEnterprise = () => {
  cfg.mobileDeviceManagement = false;
  cfg.isEnterprise = true;
  const ctx = createTeleportContextE();
  return renderPluginEnroll('', cfg.getIntegrationEnrollRoute('okta'), ctx);
};

export const EnrollOktaEnterpriseWithCleanUp = () => {
  cfg.mobileDeviceManagement = true;
  cfg.isEnterprise = true;
  const ctx = createTeleportContextE();
  return renderPluginEnroll('', cfg.getIntegrationEnrollRoute('okta'), ctx);
};
EnrollOktaEnterpriseWithCleanUp.parameters = {
  msw: {
    handlers: [
      rest.get(ecfg.api.pluginNeedsCleanupPath, async (req, res, ctx) => {
        return res(ctx.json({ needsCleanup: true }));
      }),
      rest.put(ecfg.api.pluginCleanupPath, async (req, res, ctx) => {
        return res(ctx.json({}));
      }),
      rest.post(ecfg.getPluginValidateUrl(), async (req, res, ctx) => {
        return res(ctx.json({}));
      }),
    ],
  },
};

export const EnrollOktaWithIgs = () => {
  cfg.isIgsEnabled = true;
  const ctx = createTeleportContextE();
  return renderPluginEnroll('', cfg.getIntegrationEnrollRoute('okta'), ctx);
};

export const SetUpScim = () => {
  cfg.isIgsEnabled = true;
  return (
    <PluginProvider selectedPlugin={oktaPlugin}>
      <MockInstalledPlugin>
        <SetUpScimComponent />
      </MockInstalledPlugin>
    </PluginProvider>
  );
};

export const ImportInitLoading = () => {
  cfg.isIgsEnabled = true;
  const ctx = createTeleportContext();

  return (
    <ContextProvider ctx={ctx}>
      <PluginProvider selectedPlugin={oktaPlugin}>
        <MockInstalledPlugin>
          <ImportComponent />
        </MockInstalledPlugin>
      </PluginProvider>
    </ContextProvider>
  );
};
ImportInitLoading.parameters = {
  msw: {
    handlers: [
      rest.post(ecfg.api.okta.groups, async (req, res, ctx) => {
        return res(ctx.delay('infinite'));
      }),
      rest.post(ecfg.api.okta.apps, async (req, res, ctx) => {
        return res(ctx.delay('infinite'));
      }),
      rest.get(cfg.api.usersPath, (req, res, ctx) =>
        res(ctx.delay('infinite'))
      ),
    ],
  },
};

export const Import = () => {
  cfg.isIgsEnabled = true;
  const ctx = createTeleportContext();

  return (
    <ContextProvider ctx={ctx}>
      <PluginProvider selectedPlugin={oktaPlugin}>
        <MockInstalledPlugin>
          <Info>
            Devs, to test filter use values: `test-err` and `test-query`
          </Info>
          <ImportComponent />
        </MockInstalledPlugin>
      </PluginProvider>
    </ContextProvider>
  );
};
Import.parameters = {
  msw: {
    handlers: [
      rest.post(ecfg.api.okta.groups, async (req, res, ctx) => {
        if (req.body['groupFilters']?.includes('test-err')) {
          return res(
            ctx.status(400),
            ctx.json({
              error: {
                message: 'bad filter: test-err',
              },
            })
          );
        }
        if (req.body['groupFilters']?.includes('test-query')) {
          return res(
            ctx.json([
              { name: 'group-4-test', description: 'group 4 desc' },
              { name: 'group-3-test', description: 'group 3 desc' },
            ])
          );
        }
        return res(
          ctx.json([
            { name: 'group-4', description: 'group 4 desc' },
            { name: 'group-3', description: 'group 3 desc' },
            { name: 'group-1', description: 'group 1 desc' },
            { name: 'group-5', description: 'group 5 desc' },
            { name: 'group-6', description: 'group 6 desc' },
            { name: 'group-2', description: 'group 2 desc' },
            { name: 'group-64', description: 'group 64 desc' },
            { name: 'group-63', description: 'group 63 desc' },
            { name: 'group-61', description: 'group 61 desc' },
            { name: 'group-65', description: 'group 65 desc' },
            { name: 'group-66', description: 'group 66 desc' },
            { name: 'group-62', description: 'group 62 desc' },
          ])
        );
      }),
      rest.post(ecfg.api.okta.apps, async (req, res, ctx) => {
        if (req.body['appFilters']?.includes('test-err')) {
          return res(
            ctx.status(400),
            ctx.json({
              error: {
                message: 'bad filter: test-err',
              },
            })
          );
        }
        if (req.body['appFilters']?.includes('test-query')) {
          return res(
            ctx.json([
              { name: '[staging] Cloud Platform Access - test' },
              { name: 'Zoom - test' },
            ])
          );
        }
        return res(
          ctx.json([
            { name: '1Password' },
            { name: 'Airbase' },
            { name: 'Anthem' },
            { name: 'Asana' },
            { name: 'Amazon Web Services' },
            { name: 'Bonusly' },
            { name: '[staging] Cloud Platform Access' },
            { name: 'Loom (Google Auth)' },
            { name: 'Zendesk Customer Success' },
            { name: 'Secure Code Warrior Testing Long App' },
            { name: 'Zoom' },
            { name: 'Slab' },
          ])
        );
      }),
      rest.get(cfg.api.usersPath, (req, res, ctx) =>
        res(
          ctx.json([
            { name: 'user-3', roles: ['admin'], authType: 'local' },
            { name: 'user-2', roles: ['access'], authType: 'local' },
            { name: 'user-1', roles: ['editor'], authType: 'local' },
          ])
        )
      ),
      rest.post(ecfg.api.pluginPath, (req, res, ctx) => res(ctx.json({}))),
    ],
  },
};

const MockInstalledPlugin: React.FC<PropsWithChildren> = ({ children }) => {
  const { setInstalledPlugin, installedPlugin, setFormData } =
    usePlugin<PluginOktaSpec>();
  useEffect(() => {
    setInstalledPlugin({
      kind: 'okta',
      resourceType: 'plugin',
      name: 'okta',
      statusCode: IntegrationStatusCode.Running,
      spec: {
        scimBearerToken: 'scim-bearer-token',
        oktaAppId: 'okta-app-id',
        oktaAppName: 'okta-app-name',
        teleportSsoConnector: 'teleport-sso-connector',
        error: '',
      },
    });

    const formData = new FormData();
    formData.append('orgURL', 'https://okta-test.okta.com/');
    setFormData(formData);
  }, []);

  if (installedPlugin) {
    return <>{children}</>;
  }
  return null;
};

export const Finished = () => {
  cfg.isIgsEnabled = true;

  return (
    <MemoryRouter>
      <PluginProvider selectedPlugin={oktaPlugin}>
        <PluginEnrollSuccess />
      </PluginProvider>
    </MemoryRouter>
  );
};
