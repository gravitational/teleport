import { StoryObj } from '@storybook/react-vite';
import { http, HttpResponse } from 'msw';
import { useEffect } from 'react';
import { MemoryRouter } from 'react-router';

import { InfoGuidePanelProvider } from 'shared/components/SlidingSidePanel/InfoGuide';

import cfg from 'e-teleport/config';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import TeleportEContext from 'e-teleport/teleportContextE';
import { ContextProvider } from 'teleport';
import { ContentMinWidth } from 'teleport/Main/Main';
import { allAccessAcl, noAccess } from 'teleport/mocks/contexts';
import {
  IntegrationStatusCode,
  PluginKind,
} from 'teleport/services/integrations';

import { IntegrationPick } from './IntegrationPick';

const onboardSupportPluginKinds: PluginKind[] = [
  'slack',
  'okta',
  'opsgenie',
  'jamf',
  'intune',
  'servicenow',
  'jira',
  'pagerduty',
  'email',
  'discord',
  'mattermost',
  'entra-id',
  'datadog',
  'msteams',
  'aws-identity-center',
];

const defaultIsCloudFlag = cfg.oss.isCloud;
const defaultEasEntitlement = cfg.oss.entitlements.ExternalAuditStorage;
const defaultIdentity = cfg.oss.entitlements.Identity;

export default {
  title: 'TeleportE/Integrations/Picker',
  decorators: [
    Story => {
      cfg.oss.isCloud = true;
      useEffect(() => {
        // Clean up
        return () => {
          cfg.oss.isCloud = defaultIsCloudFlag;
          cfg.oss.entitlements.ExternalAuditStorage = defaultEasEntitlement;
          cfg.oss.entitlements.Identity = defaultIdentity;
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
        http.get(cfg.api.plugin.list, () => {
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
        http.get(cfg.api.plugin.list, () => {
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
        http.get(cfg.api.plugin.list, () => {
          return HttpResponse.json(mockGetPluginsReply);
        }),
      ],
    },
  },
  render() {
    cfg.oss.entitlements.ExternalAuditStorage = { enabled: false, limit: 0 };
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
        http.get(cfg.api.plugin.list, () => {
          return HttpResponse.json(mockGetPluginsReply);
        }),
      ],
    },
  },
  render() {
    cfg.oss.entitlements.ExternalAuditStorage = { enabled: true, limit: 0 };
    cfg.oss.entitlements.Identity = { enabled: true, limit: 0 };
    const ctx = createTeleportContextE();

    return render(ctx);
  },
};

function render(ctx: TeleportEContext) {
  return (
    <MemoryRouter>
      <InfoGuidePanelProvider>
        <ContentMinWidth>
          <ContextProvider ctx={ctx}>
            <IntegrationPick />
          </ContextProvider>
        </ContentMinWidth>
      </InfoGuidePanelProvider>
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
  {
    name: 'plugin-name4',
    details: 'some detail',
    type: 'aws-identity-center',
    statusCode: IntegrationStatusCode.Running,
  },
];
