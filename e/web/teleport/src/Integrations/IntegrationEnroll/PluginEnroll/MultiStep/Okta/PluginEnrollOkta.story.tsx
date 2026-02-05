import { Meta, StoryObj } from '@storybook/react-vite';
import { http, HttpResponse } from 'msw';
import { useEffect, type ComponentType as ReactComponentType } from 'react';
import { MemoryRouter, Route } from 'react-router';

import cfg from 'e-teleport/config';
import PluginEnroll from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll';
import { OktaIntegrationSetUpContextProvider } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/SetUpContext';
import {
  APP_GROUP_SYNC_CONFIG,
  IDENTITY_SECURITY_SYNC_CONFIG,
  OktaIntegrationStepType,
  OktaSetupStepComplete,
  SCIM_CONFIG,
  SSO_CONFIG,
  USER_SYNC_CONFIG,
  type OktaIntegrationLevelStep,
  type OktaIntegrationStepWithEnabled,
} from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/Shared';
import {
  AppGroupSyncForm,
  SetUpAppGroupSync as AppGroupSyncSetup,
} from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/Steps/SetUpAppGroupSync';
import {
  SetupIdentitySecuritySync,
  SetupIdentitySecuritySyncForm,
} from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/Steps/SetupIdentitySecuritySync';
import {
  ScimForm,
  SetUpScim as ScimSetup,
} from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/Steps/SetUpScim';
import { SetUpSSO as SSOSetup } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/Steps/SetUpSSO';
import {
  UserSyncForm,
  SetUpUserSync as UserSyncSetup,
} from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/Steps/SetUpUserSync';
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

export const OverviewNoStepsComplete = {
  parameters: {
    msw: [
      http.get(cfg.getPluginUrl('okta', 'get'), async () => {
        return HttpResponse.json({});
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

export const OverviewAllStepsComplete = {
  parameters: {
    msw: [
      http.get(cfg.getPluginUrl('okta', 'get'), async () => {
        return HttpResponse.json(StubPluginAllStepsComplete);
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

export const OverviewAllExceptAppGroupSyncComplete = {
  parameters: {
    msw: [
      http.get(cfg.getPluginUrl('okta', 'get'), async () => {
        return HttpResponse.json(StubPluginAllExceptAppGroupSyncComplete);
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
      <RenderStep
        step={OktaIntegrationStepType.Sso}
        plugin={StubPluginNotSetUp}
      />
    );
  },
};

export const SetUpScim = {
  argTypes: {
    isEditing: {
      control: { type: 'boolean' },
      description: 'Enable edit view',
    },
  },
  args: { isEditing: false },
  render: args => {
    cfg.oss.entitlements.Identity = {
      enabled: (args as { hasIdentity: boolean }).hasIdentity,
      limit: 0,
    };
    return (
      <RenderStep
        step={OktaIntegrationStepType.Scim}
        plugin={StubPluginSSOSetUp}
        isEditing={args.isEditing}
      />
    );
  },
};

export const SetUpUserSync = {
  argTypes: {
    isEditing: {
      control: { type: 'boolean' },
      description: 'Enable edit view',
    },
    hasSetClientID: {
      control: { type: 'boolean' },
      description: 'If clientID has been previously set',
    },
  },
  args: { isEditing: false, hasSetClientID: false },
  render: args => {
    cfg.oss.entitlements.Identity = {
      enabled: (args as { hasIdentity: boolean }).hasIdentity,
      limit: 0,
    };
    return (
      <RenderStep
        step={OktaIntegrationStepType.UserSync}
        plugin={StubPluginSCIMSetUp}
        isEditing={args.isEditing || args.hasSetClientID}
        hasSetClientID={args.hasSetClientID}
      />
    );
  },
};

export const SetUpAppGroupSync = {
  argTypes: {
    isEditing: {
      control: { type: 'boolean' },
      description: 'Enable edit view',
    },
  },
  args: { isEditing: false },
  render: args => {
    cfg.oss.entitlements.Identity = {
      enabled: (args as { hasIdentity: boolean }).hasIdentity,
      limit: 0,
    };
    return (
      <RenderStep
        step={OktaIntegrationStepType.AppGroupSync}
        plugin={StubPluginSCIMSetUp}
        isEditing={args.isEditing}
      />
    );
  },
};

export const SetupCompleteWithAppGroupSync = {
  render: args => {
    cfg.oss.entitlements.Identity = {
      enabled: (args as { hasIdentity: boolean }).hasIdentity,
      limit: 0,
    };
    return (
      <RenderStepComplete
        config={APP_GROUP_SYNC_CONFIG}
        plugin={StubPluginAppGroupSyncEnabled}
      />
    );
  },
};

export const SetupCompleteWithoutAppGroupSync = {
  render: args => {
    cfg.oss.entitlements.Identity = {
      enabled: (args as { hasIdentity: boolean }).hasIdentity,
      limit: 0,
    };
    return (
      <RenderStepComplete
        config={APP_GROUP_SYNC_CONFIG}
        plugin={StubPluginSSOSetUp}
      />
    );
  },
};

export const StepCompleteWithNextStep = {
  render: args => {
    cfg.oss.entitlements.Identity = {
      enabled: (args as { hasIdentity: boolean }).hasIdentity,
      limit: 0,
    };
    return (
      <RenderStepComplete
        config={SSO_CONFIG}
        plugin={StubPluginSSOSetUp}
        hasNextStep
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
        oktaGroupEveryoneMappedRoles: ['some-role'],
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

const StubPluginAppGroupSyncEnabled = {
  ...StubPluginSCIMSetUp,
  spec: {
    ...StubPluginSCIMSetUp.spec,
    enableAppGroupSync: true,
  },
} satisfies Plugin<PluginOktaSpec, PluginStatusOkta>;

const StubPluginAllStepsComplete = {
  ...StubPluginSCIMSetUp,
  spec: {
    ...StubPluginSCIMSetUp.spec,
    enableAppGroupSync: true,
    enableAccessListSync: true,
    enableUserSync: true,
    enableSystemLogExport: true,
    credentialsInfo: {
      hasSCIMToken: true,
      hasConfiguredOauthCredentials: true,
    },
  },
  status: {
    ...StubPluginSCIMSetUp.status,
    details: {
      ...StubPluginSCIMSetUp.status.details,
      accessListsSyncDetails: {
        enabled: true,
        statusCode: 1,
        lastSuccess: new Date(Date.now() - 1000 * 60),
        lastFailed: new Date(0),
        numApps: 5,
        numGroups: 10,
        appFilters: [],
        groupFilters: [],
        error: '',
      },
    },
  },
} satisfies Plugin<PluginOktaSpec, PluginStatusOkta>;

const StubPluginAllExceptAppGroupSyncComplete = {
  ...StubPluginSCIMSetUp,
  spec: {
    ...StubPluginSCIMSetUp.spec,
    enableUserSync: true,
    enableSystemLogExport: true,
    credentialsInfo: {
      hasSCIMToken: true,
      hasConfiguredOauthCredentials: true,
    },
  },
} satisfies Plugin<PluginOktaSpec, PluginStatusOkta>;

type ComponentProps<T extends boolean> = T extends false
  ? object
  : {
      isEditing: boolean;
      plugin: Plugin<PluginOktaSpec, PluginStatusOkta>;
      setPlugin: (plugin: Plugin<PluginOktaSpec, PluginStatusOkta>) => void;
    };

const steps = [
  SSO_CONFIG,
  SCIM_CONFIG,
  USER_SYNC_CONFIG,
  IDENTITY_SECURITY_SYNC_CONFIG,
  APP_GROUP_SYNC_CONFIG,
].map(step => ({ ...step, enabled: true }) as OktaIntegrationStepWithEnabled);

const RenderStep = ({
  step,
  plugin,
  isEditing = false,
  hasSetClientID = false,
}: {
  step: OktaIntegrationStepType;
  plugin: Plugin<PluginOktaSpec, PluginStatusOkta>;
  isEditing?: boolean;
  hasSetClientID?: boolean;
}) => {
  const ctx = useTeleportE();

  plugin.spec.credentialsInfo.hasConfiguredOauthCredentials = hasSetClientID;

  type StepComponent = ReactComponentType<ComponentProps<typeof isEditing>>;

  let Component: StepComponent;
  switch (step) {
    default:
    case OktaIntegrationStepType.Sso:
      Component = SSOSetup;
      break;
    case OktaIntegrationStepType.Scim:
      Component = isEditing ? ScimForm : ScimSetup;
      break;
    case OktaIntegrationStepType.UserSync:
      Component = isEditing ? UserSyncForm : UserSyncSetup;
      break;
    case OktaIntegrationStepType.AppGroupSync:
      Component = isEditing ? AppGroupSyncForm : AppGroupSyncSetup;
      break;
    case OktaIntegrationStepType.IdentitySecuritySync:
      Component = isEditing
        ? SetupIdentitySecuritySyncForm
        : SetupIdentitySecuritySync;
      break;
  }

  return (
    <MemoryRouter
      initialEntries={[{ pathname: cfg.oss.getIntegrationEnrollRoute('okta') }]}
    >
      <Route path={cfg.oss.routes.integrationEnroll}>
        <ContextProvider ctx={ctx}>
          <OktaIntegrationSetUpContextProvider
            completedStepTypes={[]}
            plugin={plugin}
            startFrom={undefined}
            steps={steps}
            key={`${step}${isEditing}${hasSetClientID}`}
          >
            {isEditing ? (
              <Component isEditing plugin={plugin} setPlugin={() => {}} />
            ) : (
              <Component />
            )}
          </OktaIntegrationSetUpContextProvider>
        </ContextProvider>
      </Route>
    </MemoryRouter>
  );
};

const RenderStepComplete = ({
  config,
  plugin,
  hasNextStep = false,
}: {
  config: OktaIntegrationLevelStep;
  plugin: Plugin<PluginOktaSpec, PluginStatusOkta>;
  hasNextStep?: boolean;
}) => {
  const ctx = useTeleportE();

  const stepsToUse = hasNextStep
    ? steps
    : steps.slice(0, steps.findIndex(s => s.type === config.type) + 1);

  return (
    <MemoryRouter
      initialEntries={[{ pathname: cfg.oss.getIntegrationEnrollRoute('okta') }]}
    >
      <Route path={cfg.oss.routes.integrationEnroll}>
        <ContextProvider ctx={ctx}>
          <OktaIntegrationSetUpContextProvider
            completedStepTypes={[]}
            plugin={plugin}
            startFrom={undefined}
            steps={stepsToUse}
          >
            <OktaSetupStepComplete config={config} />
          </OktaIntegrationSetUpContextProvider>
        </ContextProvider>
      </Route>
    </MemoryRouter>
  );
};
