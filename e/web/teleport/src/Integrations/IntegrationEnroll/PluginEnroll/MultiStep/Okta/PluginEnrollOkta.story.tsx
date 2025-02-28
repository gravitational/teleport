import { Meta, StoryObj } from '@storybook/react';
import { http, HttpResponse } from 'msw';
import { useEffect } from 'react';
import { MemoryRouter, Route } from 'react-router';

import cfg from 'e-teleport/config';
import PluginEnroll from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll';
import { OktaIntegrationSetUpContextProvider } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/SetUpContext';
import { OktaIntegrationLevel } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/Shared';
import { SetUpAppGroupSync as AppGroupSyncSetup } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/Steps/SetUpAppGroupSync';
import { SetUpScim as ScimSetup } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/Steps/SetUpScim';
import { SetUpSSO as SSOSetup } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/Steps/SetUpSSO';
import { SetUpUserSync as UserSyncSetup } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/Steps/SetUpUserSync';
import ResourceServiceE from 'e-teleport/services/resource';
import useTeleportE from 'e-teleport/useTeleportE';
import { ContextProvider } from 'teleport';
import { createTeleportContext } from 'teleport/mocks/contexts';
import {
  IntegrationStatusCode,
  PluginOktaSpec,
  type Plugin,
} from 'teleport/services/integrations';
import { PluginStatusOkta } from 'teleport/services/integrations/oktaStatusTypes';
import {
  DefaultAuthConnector,
  KindAuthConnectors,
  Resource,
} from 'teleport/services/resources';

const defaultIdentity = cfg.oss.entitlements.Identity;

const stubResourceService = {
  fetchAuthConnectors: () => {
    return Promise.resolve({
      connectors: [],
      defaultConnector: undefined,
    }) satisfies Promise<{
      defaultConnector: DefaultAuthConnector;
      connectors: Resource<KindAuthConnectors>[];
    }>;
  },
} as ResourceServiceE;

export default {
  title: 'TeleportE/Integrations/Enroll/Okta',
  decorators: [
    Story => {
      const ctx = createTeleportContext();
      ctx.entitlements.Identity = { enabled: true, limit: 0 };
      ctx.resourceService = stubResourceService;

      useEffect(() => {
        // Clean up
        return () => {
          cfg.oss.entitlements.Identity = defaultIdentity;
        };
      }, []);

      return (
        <MemoryRouter
          initialEntries={[
            { pathname: cfg.oss.getIntegrationEnrollRoute('okta') },
          ]}
        >
          <Route path={cfg.oss.routes.integrationEnroll}>
            <ContextProvider ctx={ctx}>
              <Story />
            </ContextProvider>
          </Route>
        </MemoryRouter>
      );
    },
  ],
  argTypes: {
    hasIdentity: {
      control: { type: 'boolean' },
      description: 'Enable Identity entitlement',
    },
  },
  args: { hasIdentity: true },
} satisfies Meta<typeof PluginEnroll>;

export const Overview = {
  parameters: {
    msw: [
      http.get(cfg.getPluginUrl('okta'), async () => {
        return HttpResponse.json();
      }),
    ],
  },
  render: args => {
    cfg.oss.entitlements.Identity = {
      enabled: (args as { hasIdentity: boolean }).hasIdentity,
      limit: 0,
    };
    return <PluginEnroll />;
  },
} satisfies StoryObj<typeof PluginEnroll>;

export const SetUpSSO = {
  render: args => {
    cfg.oss.entitlements.Identity = {
      enabled: (args as { hasIdentity: boolean }).hasIdentity,
      limit: 0,
    };
    return (
      <RenderStep step={OktaIntegrationLevel.SSO} plugin={StubPluginNotSetUp} />
    );
  },
};

export const SetUpScim = {
  render: args => {
    cfg.oss.entitlements.Identity = {
      enabled: (args as { hasIdentity: boolean }).hasIdentity,
      limit: 0,
    };
    return (
      <RenderStep
        step={OktaIntegrationLevel.SCIM}
        plugin={StubPluginSSOSetUp}
      />
    );
  },
};

export const SetUpUserSync = {
  render: args => {
    cfg.oss.entitlements.Identity = {
      enabled: (args as { hasIdentity: boolean }).hasIdentity,
      limit: 0,
    };
    return (
      <RenderStep
        step={OktaIntegrationLevel.USER_SYNC}
        plugin={StubPluginSCIMSetUp}
      />
    );
  },
};

export const SetUpAppGroupSync = {
  render: args => {
    cfg.oss.entitlements.Identity = {
      enabled: (args as { hasIdentity: boolean }).hasIdentity,
      limit: 0,
    };
    return (
      <RenderStep
        step={OktaIntegrationLevel.APP_GROUP_SYNC}
        plugin={StubPluginSCIMSetUp}
      />
    );
  },
};

const StubPluginNotSetUp = {
  name: 'okta',
  kind: 'okta',
  resourceType: 'plugin',
  spec: {
    teleportSsoConnector: 'okta-integration',
    orgUrl: undefined,
    defaultOwners: [],
    oktaAppId: 'a94a8fe5c-cb19ba61c',
    oktaAppName: undefined,
    scimBearerToken: undefined,
    error: undefined,
  },
  statusCode: IntegrationStatusCode.Running,
  status: {
    code: IntegrationStatusCode.Running,
    lastRun: new Date(Date.now() - 1000 * 60),
    errorMessage: undefined,
    details: {},
  },
} satisfies Plugin<PluginOktaSpec, PluginStatusOkta>;

const StubPluginSSOSetUp = {
  ...StubPluginNotSetUp,
  spec: {
    ...StubPluginNotSetUp.spec,
    orgUrl: 'https://dev-testing.okta.com',
  },
  status: {
    ...StubPluginNotSetUp.status,
    details: {
      ssoDetails: {
        appName: undefined,
        appId: 'a94a8fe5c-cb19ba61c',
        enabled: true,
      },
    },
  },
} satisfies Plugin<PluginOktaSpec, PluginStatusOkta>;

const StubPluginSCIMSetUp = {
  ...StubPluginSSOSetUp,
  spec: {
    ...StubPluginSSOSetUp.spec,
    credentialsInfo: {
      hasSCIMToken: true,
    },
  },
  status: {
    ...StubPluginSSOSetUp.status,
    details: {
      ...StubPluginSSOSetUp.status.details,
      scimDetails: {
        enabled: true,
      },
    },
  },
} satisfies Plugin<PluginOktaSpec, PluginStatusOkta>;

const RenderStep = ({
  step,
  plugin,
}: {
  step: OktaIntegrationLevel;
  plugin: Plugin<PluginOktaSpec, PluginStatusOkta>;
}) => {
  const ctx = useTeleportE();
  let Component = () => <div>Unknown step</div>;
  switch (step) {
    case OktaIntegrationLevel.SSO:
      Component = SSOSetup;
      break;
    case OktaIntegrationLevel.SCIM:
      Component = ScimSetup;
      break;
    case OktaIntegrationLevel.USER_SYNC:
      Component = UserSyncSetup;
      break;
    case OktaIntegrationLevel.APP_GROUP_SYNC:
      Component = AppGroupSyncSetup;
      break;
  }

  return (
    <MemoryRouter
      initialEntries={[{ pathname: cfg.oss.getIntegrationEnrollRoute('okta') }]}
    >
      <Route path={cfg.oss.routes.integrationEnroll}>
        <ContextProvider ctx={ctx}>
          <OktaIntegrationSetUpContextProvider
            plugin={plugin}
            setPlugin={() => {}}
            startFrom={undefined}
            key={step}
          >
            <Component />
          </OktaIntegrationSetUpContextProvider>
        </ContextProvider>
      </Route>
    </MemoryRouter>
  );
};
