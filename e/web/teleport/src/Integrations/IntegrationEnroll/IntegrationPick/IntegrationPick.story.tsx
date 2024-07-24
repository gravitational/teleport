import React, { useEffect } from 'react';
import { MemoryRouter } from 'react-router';
import { ContextProvider } from 'teleport';
import {
  IntegrationStatusCode,
  PluginKind,
} from 'teleport/services/integrations';
import { noAccess, allAccessAcl } from 'teleport/mocks/contexts';

import { StoryObj } from '@storybook/react';

import { http, HttpResponse } from 'msw';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import cfg from 'e-teleport/config';
import TeleportEContext from 'e-teleport/teleportContextE';

import { IntegrationPick } from './IntegrationPick';

const onboardSupportPluginKinds: PluginKind[] = [
  'slack',
  'okta',
  'opsgenie',
  'jamf',
  'entra-id',
];

const defaultIsCloudFlag = cfg.oss.isCloud;
const defaultMdmFlag = cfg.oss.mobileDeviceManagement;
const defaultEasFlag = cfg.oss.externalAuditStorage;
const defaultIsEnterprise = cfg.oss.isEnterprise;
const defaultIsIgsEnabled = cfg.oss.isIgsEnabled;

export default {
  title: 'TeleportE/Integrations/Picker',
  decorators: [
    Story => {
      cfg.oss.isCloud = true;
      cfg.oss.isEnterprise = true;
      useEffect(() => {
        // Clean up
        return () => {
          cfg.oss.isCloud = defaultIsCloudFlag;
          cfg.oss.mobileDeviceManagement = defaultMdmFlag;
          cfg.oss.externalAuditStorage = defaultEasFlag;
          cfg.oss.isEnterprise = defaultIsEnterprise;
          cfg.oss.isIgsEnabled = defaultIsIgsEnabled;
        };
      }, []);

      return <Story />;
    },
  ],
};

export const NoPluginsEnrolled: StoryObj = {
  parameters: {
    msw: {
      handlers: [
        http.get(cfg.api.pluginTypesPath, () => {
          return HttpResponse.json(onboardSupportPluginKinds);
        }),
        http.get(cfg.getPluginUrl(), () => {
          return HttpResponse.json([]);
        }),
      ],
    },
  },
  render() {
    const ctx = createTeleportContextE();
    return render(ctx);
  },
};

export const PluginsEnrolled: StoryObj = {
  parameters: {
    msw: {
      handlers: [
        http.get(cfg.api.pluginTypesPath, () => {
          return HttpResponse.json(onboardSupportPluginKinds);
        }),
        http.get(cfg.getPluginUrl(), () => {
          return HttpResponse.json(mockGetPluginsReply);
        }),
      ],
    },
  },
  render() {
    const ctx = createTeleportContextE();

    return render(ctx);
  },
};

export const Error: StoryObj = {
  parameters: {
    msw: {
      handlers: [
        http.get(cfg.api.pluginTypesPath, () => {
          return HttpResponse.json(
            {
              message: 'some error message',
            },
            { status: 500 }
          );
        }),
      ],
    },
  },
  render() {
    const ctx = createTeleportContextE();

    return render(ctx);
  },
};

export const NoAccess = () => {
  const ctx = createTeleportContextE({
    customAcl: {
      ...allAccessAcl,
      plugins: noAccess,
      integrations: { ...noAccess, use: false },
    },
  });

  return render(ctx);
};

export const RequiresEnterprise: StoryObj = {
  parameters: {
    msw: {
      handlers: [
        http.get(cfg.api.pluginTypesPath, () => {
          return HttpResponse.json(onboardSupportPluginKinds);
        }),
        http.get(cfg.getPluginUrl(), () => {
          return HttpResponse.json(mockGetPluginsReply);
        }),
      ],
    },
  },
  render() {
    cfg.oss.isTeam = true;
    const ctx = createTeleportContextE();

    return render(ctx);
  },
};

export const FullFeatures: StoryObj = {
  parameters: {
    msw: {
      handlers: [
        http.get(cfg.api.pluginTypesPath, () => {
          return HttpResponse.json(onboardSupportPluginKinds);
        }),
        http.get(cfg.getPluginUrl(), () => {
          return HttpResponse.json(mockGetPluginsReply);
        }),
      ],
    },
  },
  render() {
    cfg.oss.mobileDeviceManagement = true;
    cfg.oss.externalAuditStorage = true;
    cfg.oss.isEnterprise = true;
    cfg.oss.isIgsEnabled = true;
    const ctx = createTeleportContextE();

    return render(ctx);
  },
};

function render(ctx: TeleportEContext) {
  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <IntegrationPick />
      </ContextProvider>
    </MemoryRouter>
  );
}

const mockGetPluginsReply = [
  {
    name: 'plugin-name',
    details: 'some detail',
    type: 'slack',
    statusCode: IntegrationStatusCode.Running,
  },
  {
    name: 'plugin-name2',
    details: 'some detail2',
    type: 'okta',
    statusCode: IntegrationStatusCode.Running,
  },
  {
    name: 'plugin-name3',
    details: 'some detail3',
    type: 'opsgenie',
    statusCode: IntegrationStatusCode.Running,
  },
];
