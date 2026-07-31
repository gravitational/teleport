import { ComponentProps, PropsWithChildren, ReactNode } from 'react';
import { Link } from 'react-router';
import styled from 'styled-components';

import {
  Box,
  ButtonPrimary,
  ButtonSecondary,
  Flex,
  H2,
  Image,
  Text,
} from 'design';
import pamSuccess from 'design/assets/images/icons/success.png';
import { FeatureName } from 'design/constants';
import * as Icons from 'design/Icon';
import { HoverTooltip } from 'design/Tooltip';

import { goToCreateAccessListFromOktaRoute } from 'e-teleport/AccessListManagement/CreateAccessList/route';
import cfg from 'e-teleport/config';
import { useOktaIntegrationSetUpContext } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/SetUpContext';
import { FormDataField } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/types';
import { pluginsService } from 'e-teleport/services/plugins';
import type { Plugin, PluginOktaSpec } from 'teleport/services/integrations';
import { PluginStatusOkta } from 'teleport/services/integrations/oktaStatusTypes';

export const getCompletedOktaIntegrationStepTypes = (
  plugin: Partial<Plugin<PluginOktaSpec, PluginStatusOkta>> | undefined
) => {
  const completed: OktaIntegrationStepType[] = [];

  if (!plugin?.spec) {
    return completed;
  }

  // Although App/Group Sync and AccessList Sync should both be enabled/disabled,
  // there may be cases where App/Group Sync is enabled, while AccessList Sync is not.
  // This should be treated as disabled/not setup.
  if (
    plugin.spec.enableAccessListSync ||
    plugin.status?.details?.accessListsSyncDetails?.enabled
  ) {
    completed.push(OktaIntegrationStepType.AppGroupSync);
  }
  if (
    plugin.spec.enableUserSync &&
    plugin.spec.credentialsInfo?.hasConfiguredOauthCredentials
  ) {
    completed.push(OktaIntegrationStepType.UserSync);
  }
  if (
    plugin.spec.credentialsInfo?.hasSCIMToken ||
    plugin.status?.details?.scimDetails?.enabled
  ) {
    completed.push(OktaIntegrationStepType.Scim);
  }
  if (plugin.spec.orgUrl) {
    completed.push(OktaIntegrationStepType.Sso);
  }
  if (plugin.spec.enableSystemLogExport) {
    completed.push(OktaIntegrationStepType.IdentitySecuritySync);
  }

  return completed;
};

export const OktaSetupStepComplete = ({
  config,
}: {
  config: OktaIntegrationLevelStep;
}) => {
  const { steps, getNextStep, plugin, preservedLocationState } =
    useOktaIntegrationSetUpContext();

  const nextStep = getNextStep(config.type);

  let finalSteps: React.ReactNode;

  if (!nextStep) {
    if (plugin?.spec?.enableAppGroupSync) {
      finalSteps = (
        <>
          <HoverTooltip
            tipContent="Set up access to Teleport protected resources for your Okta user
              groups."
          >
            <ButtonPrimary
              as={Link}
              {...goToCreateAccessListFromOktaRoute(
                plugin.spec.orgUrl,
                preservedLocationState?.preset
                  ? preservedLocationState
                  : undefined
              )}
            >
              {preservedLocationState?.preset
                ? 'Finish setting up access'
                : 'Set Up Access'}
            </ButtonPrimary>
          </HoverTooltip>
          <ButtonSecondary
            as={Link}
            to={cfg.oss.getIntegrationStatusRoute('okta', 'okta')}
          >
            See the Integration Status Page
          </ButtonSecondary>
        </>
      );
    } else {
      finalSteps = (
        <ButtonPrimary
          as={Link}
          to={cfg.oss.getIntegrationStatusRoute('okta', 'okta')}
        >
          See the Integration Status Page
        </ButtonPrimary>
      );
    }
  }

  const index = steps
    .filter(step => step.enabled)
    .findIndex(step => step.type === config.type);

  const { completeCopy } = config;

  const body =
    typeof completeCopy.body === 'function'
      ? completeCopy.body(index + 1, nextStep?.type)
      : completeCopy.body;

  return (
    <Flex flexDirection="column" alignItems="center" mt={6} gap={2}>
      <Image src={pamSuccess} maxWidth="120px" />
      <H2>{completeCopy.title}</H2>
      <Box maxWidth="500px" textAlign="center" mb={1}>
        {body}
      </Box>
      <Flex flexDirection="row" alignItems="center" gap={3}>
        {!nextStep ? (
          <>{finalSteps}</>
        ) : (
          <>
            <ButtonPrimary
              as={Link}
              to={cfg.oss.getIntegrationEnrollRoute('okta', nextStep.type)}
            >
              Next{' - '}
              {nextStep.shortName}
            </ButtonPrimary>
            <ButtonSecondary
              as={Link}
              to={cfg.oss.getIntegrationStatusRoute('okta', 'okta')}
            >
              See the Integration Status Page
            </ButtonSecondary>
          </>
        )}
      </Flex>
    </Flex>
  );
};

export const createOktaPlugin = ({
  eventId,
  enableAccessListSync,
  enableAppGroupsSync,
  enableUserSync,
  reuseConnector,
  metadataUrl,
  orgUrl,
}: {
  eventId: string;
  enableAccessListSync: boolean;
  enableAppGroupsSync: boolean;
  enableUserSync: boolean;
  reuseConnector?: string;
  metadataUrl?: string;
  orgUrl?: string;
}) => {
  const formData = new FormData();
  formData.set('event_id', eventId);
  formData.set('name', 'okta-default');
  formData.set('type', 'okta');
  formData.set(
    FormDataField.EnableAccessListSync,
    enableAccessListSync.toString()
  );
  formData.set(
    FormDataField.EnableAppGroupsSync,
    enableAppGroupsSync.toString()
  );
  formData.set(FormDataField.EnableUserSync, enableUserSync.toString());
  if (metadataUrl) {
    formData.set(FormDataField.MetadataURL, metadataUrl);
  }
  if (orgUrl) {
    formData.set(FormDataField.OrgUrl, orgUrl);
  }
  if (reuseConnector) {
    formData.set(FormDataField.ReuseConnector, reuseConnector);
  }

  return pluginsService.createStaticAuthPlugin<'okta'>(formData);
};

export enum OktaIntegrationStepType {
  Sso = 'sso',
  Scim = 'scim',
  UserSync = 'user-sync',
  AppGroupSync = 'app-group-sync',
  IdentitySecuritySync = 'identity-security-sync',
}

export enum StepRestriction {
  DisabledInCloud,
}

export enum OktaLevelProductRequirement {
  IdentitySecurity = FeatureName.IdentitySecurity,
  IdentityGovernance = FeatureName.IdentityGovernance,
}

export interface OktaIntegrationLevelStep {
  bullets: ReactNode[];
  completeCopy: {
    body:
      | ReactNode
      | ((
          level: number,
          nextStepType: OktaIntegrationStepType | undefined
        ) => ReactNode);
    title: string;
  };
  type: OktaIntegrationStepType;
  productRequirement?: OktaLevelProductRequirement;
  restriction?: StepRestriction;
  name: string;
  shortName: string;
}

export interface OktaIntegrationStepWithEnabled extends OktaIntegrationLevelStep {
  enabled: boolean;
}

function calculateIsEnabled(
  config: OktaIntegrationLevelStep,
  activityCenterEnabled: boolean
): boolean {
  if (!config.productRequirement) {
    return true;
  }

  switch (config.productRequirement) {
    case OktaLevelProductRequirement.IdentityGovernance:
      return cfg.oss.entitlements.Identity.enabled;

    case OktaLevelProductRequirement.IdentitySecurity:
      return activityCenterEnabled;
  }
}

function shouldIncludeStep(step: OktaIntegrationLevelStep, isCloud: boolean) {
  if (step.restriction === StepRestriction.DisabledInCloud && isCloud) {
    return false;
  }

  return true;
}

export function getOktaIntegrationSteps(
  activityCenterEnabled: boolean,
  isCloud: boolean
): OktaIntegrationStepWithEnabled[] {
  const configs: OktaIntegrationLevelStep[] = [
    SSO_CONFIG,
    SCIM_CONFIG,
    USER_SYNC_CONFIG,
    IDENTITY_SECURITY_SYNC_CONFIG,
    APP_GROUP_SYNC_CONFIG,
  ];

  return configs
    .filter(config => shouldIncludeStep(config, isCloud))
    .map(config => ({
      ...config,
      enabled: calculateIsEnabled(config, activityCenterEnabled),
    }));
}

export const SSO_CONFIG: OktaIntegrationLevelStep = {
  type: OktaIntegrationStepType.Sso,
  name: 'Single Sign-on (SSO)',
  shortName: 'Single Sign-on',
  completeCopy: {
    title: 'SSO Connected!',
    body: level =>
      cfg.oss.entitlements.Identity.enabled ? (
        <Text>
          <i>Level {level + 1}: SCIM</i> will enable real-time user updates so
          that Okta and Teleport always remain in sync. We recommend that you
          keep going, but you can always pick up where you left off from the
          integration status page.
        </Text>
      ) : (
        <Text>
          SSO is now connected. Users can log into Teleport using Okta.
        </Text>
      ),
  },
  bullets: [
    <Text key={1}>
      Your team can <b>log into Teleport using Okta</b>. Users will not persist
      in the system after signing out.
    </Text>,
  ],
};

export const SCIM_CONFIG: OktaIntegrationLevelStep = {
  type: OktaIntegrationStepType.Scim,
  name: 'SCIM',
  shortName: 'SCIM',
  completeCopy: {
    title: 'SCIM Configured!',
    body: level => (
      <Text>
        <i>Level {level + 1}: User Sync</i> will allow users to persist in
        Teleport after they’ve logged out. We recommend that you keep going, but
        you can always pick up where you left off from the integration status
        page.
      </Text>
    ),
  },
  bullets: [
    <Text key={1}>
      <b>Sync changes in real-time</b> with the SCIM protocol so that users
      update instantly.
    </Text>,
  ],
  productRequirement: OktaLevelProductRequirement.IdentityGovernance,
};

const appGroupsDescription = (level: number) => (
  <>
    <Text>
      <i>Level {level + 1}: Sync Apps and Groups to Teleport Access Lists</i> is
      the final piece of the Okta integration, and it will enable JIT Access
      Requests and managing access to Okta groups and apps from Teleport.
    </Text>
    <Text>
      We recommend that you keep going, but you can always pick up where you
      left off from the integration status page.
    </Text>
  </>
);

export const IDENTITY_SECURITY_SYNC_CONFIG: OktaIntegrationLevelStep = {
  type: OktaIntegrationStepType.IdentitySecuritySync,
  name: 'Identity Security Sync',
  shortName: 'Identity Security Sync',
  completeCopy: {
    title: 'Identity Security Sync Configured!',
    body: appGroupsDescription,
  },
  bullets: [
    <Text key={1}>Sync the Okta Audit Log with Identity Security</Text>,
  ],
  productRequirement: OktaLevelProductRequirement.IdentitySecurity,
  restriction: StepRestriction.DisabledInCloud,
};

export const USER_SYNC_CONFIG: OktaIntegrationLevelStep = {
  type: OktaIntegrationStepType.UserSync,
  name: 'User Sync',
  shortName: 'User Sync',
  completeCopy: {
    title: 'User Sync Configured!',
    body: (level, nextStepType) =>
      nextStepType === OktaIntegrationStepType.AppGroupSync ? (
        appGroupsDescription(level)
      ) : nextStepType === OktaIntegrationStepType.IdentitySecuritySync ? (
        <Text>
          <i>Level {level + 1}: Sync Okta's Audit Log to Identity Security</i>{' '}
          is the next step in the Okta integration. This will enable Okta's
          Audit Log to be synced to and monitored by Identity Security.
        </Text>
      ) : null,
  },
  bullets: [
    <Text key={1}>
      Users will stay synced with Okta and <b>persist after they log out</b>.
    </Text>,
  ],
  productRequirement: OktaLevelProductRequirement.IdentityGovernance,
};

export const APP_GROUP_SYNC_CONFIG: OktaIntegrationLevelStep = {
  type: OktaIntegrationStepType.AppGroupSync,
  name: 'Sync Apps and Groups to Teleport Access Lists',
  shortName: 'App and Group Sync',
  completeCopy: {
    title: 'App and Group Sync Configured!',
    body: (
      <Text>
        Please allow some time for your user groups and apps to sync to Access
        Lists. In the meantime, you can see your integration state on the Okta
        status page.
      </Text>
    ),
  },
  bullets: [
    <Text key={1}>
      Use <b>JIT Access Requests</b> to manage access to apps and user groups.
    </Text>,
    <Text key={2}>
      <b>Manage Okta group and app assignments</b> via Teleport RBAC.
    </Text>,
  ],
  productRequirement: OktaLevelProductRequirement.IdentityGovernance,
};

export const NumberedList = styled.ol`
  margin-top: 0;
  margin-bottom: 0;
  padding-left: ${({ theme }) => theme.space[3]}px;
  list-style: decimal;
`;

export const BulletList = styled.ul`
  margin-top: 0;
  margin-bottom: 0;
  padding-left: ${({ theme }) => theme.space[3]}px;
  list-style: disc;
`;

export const ListItem = ({
  children,
  ...flexProps
}: PropsWithChildren<ComponentProps<typeof Flex>>) => (
  <li>
    <Flex flexDirection="column" gap={2} mb={2} {...flexProps}>
      {children}
    </Flex>
  </li>
);

export const UpsellBulletList = ({
  bullets,
  color = 'text.muted',
}: {
  bullets: readonly ReactNode[];
  color?: string;
}) =>
  bullets.map((bullet, idx) => (
    <Flex
      key={idx}
      flexDirection="row"
      alignItems="center"
      justifyContent="start"
      gap={2}
      mt={idx === 0 ? 2 : 1}
    >
      <Icons.CircleCheck size="medium" color={color} />
      {bullet}
    </Flex>
  ));
