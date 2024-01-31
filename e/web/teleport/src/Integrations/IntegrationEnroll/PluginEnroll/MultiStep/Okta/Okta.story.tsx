import React, { useEffect } from 'react';
import { MemoryRouter } from 'react-router';

import cfg from 'teleport/config';
import {
  IntegrationStatusCode,
  PluginOktaSpec,
} from 'teleport/services/integrations';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';

import { renderPluginEnroll } from '../../PluginEnroll.story';
import { PluginProvider, usePlugin } from '../usePlugin';
import { CloudHostablePlugin, pluginMap } from '../../plugins';
import { PluginEnrollSuccess } from '../PluginEnrollSuccess';

import { SetUpScim as SetUpScimComponent } from './SetUpScim';

const oktaPlugin = pluginMap['okta'] as CloudHostablePlugin;

const defaultIsTeamFlag = cfg.isTeam;
const defaultIsEnterprise = cfg.isEnterprise;
const defaultIgs = cfg.isIgsEnabled;

export default {
  title: 'TeleportE/Integrations/Enroll/Okta',
  decorators: [
    Story => {
      useEffect(() => {
        // Clean up
        return () => {
          cfg.isTeam = defaultIsTeamFlag;
          cfg.isEnterprise = defaultIsEnterprise;
          cfg.isIgsEnabled = defaultIgs;
        };
      }, []);
      return <Story />;
    },
  ],
};

export const EnrollOktaIsTeam = () => {
  cfg.isTeam = true;
  cfg.isEnterprise = true;
  const ctx = createTeleportContextE();
  return renderPluginEnroll('', cfg.getIntegrationEnrollRoute('okta'), ctx);
};

export const EnrollOktaEnterprise = () => {
  cfg.isTeam = false;
  cfg.isEnterprise = true;
  const ctx = createTeleportContextE();
  return renderPluginEnroll('', cfg.getIntegrationEnrollRoute('okta'), ctx);
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
      <MockInstalledPlugin />
    </PluginProvider>
  );
};

const MockInstalledPlugin = () => {
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
    formData.append('orgURL', 'okta-test.com');
    setFormData(formData);
  }, []);

  if (installedPlugin) {
    return <SetUpScimComponent />;
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
