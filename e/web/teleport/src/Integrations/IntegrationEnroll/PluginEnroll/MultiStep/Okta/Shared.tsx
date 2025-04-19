import React, { ComponentProps, PropsWithChildren, ReactNode } from 'react';
import { Link } from 'react-router-dom';
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
import * as Icons from 'design/Icon';

import cfg from 'e-teleport/config';
import { FormDataField } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/types';
import { pluginsService } from 'e-teleport/services/plugins';
import { getXCSRFToken } from 'teleport/services/api';
import type { Plugin, PluginOktaSpec } from 'teleport/services/integrations';
import { PluginStatusOkta } from 'teleport/services/integrations/oktaStatusTypes';

export const getNextOktaIntegrationLevel = (
  current: OktaIntegrationLevel | undefined
) => {
  if (!current) {
    return OktaIntegrationLevel.SSO;
  }
  if (!cfg.oss.entitlements.Identity.enabled) {
    return undefined;
  }
  switch (current) {
    case OktaIntegrationLevel.SSO:
      return OktaIntegrationLevel.SCIM;
    case OktaIntegrationLevel.SCIM:
      return OktaIntegrationLevel.USER_SYNC;
    case OktaIntegrationLevel.USER_SYNC:
      return OktaIntegrationLevel.APP_GROUP_SYNC;
    case OktaIntegrationLevel.APP_GROUP_SYNC:
      return undefined;
  }
};

export const getCompletedOktaIntegrationLevel = (
  plugin: Partial<Plugin<PluginOktaSpec, PluginStatusOkta>> | undefined
) => {
  const completed = {
    [OktaIntegrationLevel.SSO]: false,
    [OktaIntegrationLevel.SCIM]: false,
    [OktaIntegrationLevel.USER_SYNC]: false,
    [OktaIntegrationLevel.APP_GROUP_SYNC]: false,
  };

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
    completed[OktaIntegrationLevel.APP_GROUP_SYNC] = true;
  }
  if (
    plugin.spec.enableUserSync ||
    plugin.spec.credentialsInfo?.hasConfiguredOauthCredentials
  ) {
    completed[OktaIntegrationLevel.USER_SYNC] = true;
  }
  if (
    plugin.spec.credentialsInfo?.hasSCIMToken ||
    plugin.status?.details?.scimDetails?.enabled
  ) {
    completed[OktaIntegrationLevel.SCIM] = true;
  }
  if (plugin.spec.orgUrl) {
    completed[OktaIntegrationLevel.SSO] = true;
  }

  return completed;
};

export const OktaSetupStepComplete = ({
  step,
}: {
  step: OktaIntegrationLevel;
}) => {
  const { completeCopy } = oktaIntegrationLevels[step];
  const nextLevel = getNextOktaIntegrationLevel(step);

  return (
    <Flex flexDirection="column" alignItems="center" mt={6} gap={2}>
      <Image src={pamSuccess} maxWidth="120px" />
      <H2>{completeCopy.title}</H2>
      <Box maxWidth="500px" textAlign="center" mb={1}>
        {completeCopy.body}
      </Box>
      <Flex flexDirection="row" alignItems="center" gap={3}>
        {!nextLevel ? (
          <ButtonPrimary
            as={Link}
            to={cfg.oss.getIntegrationStatusRoute('okta', 'okta')}
          >
            See the Integration Status Page
          </ButtonPrimary>
        ) : (
          <>
            <ButtonPrimary
              as={Link}
              to={cfg.oss.getIntegrationEnrollRoute('okta', nextLevel)}
            >
              Next{' - '}
              {oktaIntegrationLevels[nextLevel].shortName}
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
  formData.set('csrf_token', getXCSRFToken());
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

export enum OktaIntegrationLevel {
  SSO = 'sso',
  SCIM = 'scim',
  USER_SYNC = 'user-sync',
  APP_GROUP_SYNC = 'app-group-sync',
}

export const OktaIntegrationLabelValues = Object.values(OktaIntegrationLevel);

/**
 * oktaIntegrationLevelsConf is every 'level' of Okta integration that Teleport supports
 * (e.g. SSO, User Sync, App/Group Sync).
 * It contains copy for each level and each level's setup/edit flow,
 * used in the PluginEnroll process and Integration Status views.
 */
export const oktaIntegrationLevels = {
  [OktaIntegrationLevel.SSO]: {
    level: 1,
    name: 'Single Sign-on (SSO)',
    shortName: 'Single Sign-on',
    completeCopy: {
      title: 'SSO Connected!',
      body: cfg.oss.entitlements.Identity.enabled ? (
        <Text>
          <i>Level 2: SCIM</i> will enable real-time user updates so that Okta
          and Teleport always remain in sync. We recommend that you keep going,
          but you can always pick up where you left off from the integration
          status page.
        </Text>
      ) : (
        <Text>
          SSO is now connected. Users can log into Teleport using Okta.
        </Text>
      ),
    },
    bullets: [
      <Text key={1}>
        Your team can <b>log into Teleport using Okta</b>. Users will not
        persist in the system after signing out.
      </Text>,
    ],
  },
  [OktaIntegrationLevel.SCIM]: {
    level: 2,
    name: 'SCIM',
    shortName: 'SCIM',
    completeCopy: {
      title: 'SCIM Configured!',
      body: (
        <Text>
          <i>Level 3: User Sync</i> will allow users to persist in Teleport
          after they’ve logged out. We recommend that you keep going, but you
          can always pick up where you left off from the integration status
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
  },
  [OktaIntegrationLevel.USER_SYNC]: {
    level: 3,
    name: 'User Sync',
    shortName: 'User Sync',
    completeCopy: {
      title: 'User Sync Configured!',
      body: (
        <>
          <Text>
            <i>Level 4: Sync Apps and Groups to Teleport Access Lists</i> is the
            final piece of the Okta integration, and it will enable JIT Access
            Requests and managing access to Okta groups and apps from Teleport.
          </Text>
          <Text>
            We recommend that you keep going, but you can always pick up where
            you left off from the integration status page.
          </Text>
        </>
      ),
    },
    bullets: [
      <Text key={1}>
        Users will stay synced with Okta and <b>persist after they log out</b>.
      </Text>,
    ],
  },
  [OktaIntegrationLevel.APP_GROUP_SYNC]: {
    level: 4,
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
  },
} as const;

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

const StyledBoxComponent = styled(Flex)`
  position: relative;
  background-color: ${props => props.theme.colors.levels.surface};
  box-shadow:
    0 2px 1px -1px rgba(0, 0, 0, 0.2),
    0 1px 1px 0 rgba(0, 0, 0, 0.14),
    0 1px 3px 0 rgba(0, 0, 0, 0.12);
`;

export const StyledBox = ({
  header,
  children,
  ...props
}: PropsWithChildren<
  {
    header?: string | ReactNode;
  } & ComponentProps<typeof StyledBoxComponent>
>) => (
  <StyledBoxComponent
    p={4}
    gap={3}
    borderRadius={3}
    flexDirection="column"
    maxWidth="800px"
    {...props}
  >
    {typeof header === 'string' ? (
      <H2 mt={-1}>{header}</H2>
    ) : typeof header !== 'undefined' ? (
      <Box mt={-1}>{header}</Box>
    ) : null}
    {children}
  </StyledBoxComponent>
);

export const UpsellBulletList = ({
  bullets,
  color = 'text.muted',
}: {
  bullets: readonly React.JSX.Element[];
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
