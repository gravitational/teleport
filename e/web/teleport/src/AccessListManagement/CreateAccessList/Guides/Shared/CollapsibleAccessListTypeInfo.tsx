import { Box, Link as ExternalLink, Mark, Stack, Text } from 'design';
import { Alert, Info } from 'design/Alert';
import { CollapsibleInfoSection } from 'design/CollapsibleInfoSection';

import { OktaIntegrationStepType } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/Shared';
import cfg from 'teleport/config';

import { UserTypeOption } from '../../types';
import { Okta, UserCategory } from './types';

export function CollapsibleAccessListTypeInfo({
  userTypeOption,
  userCategory,
  okta,
}: {
  userTypeOption: UserTypeOption;
  userCategory: UserCategory;
  okta?: Okta;
}) {
  let Info;
  let label = '';

  if (userTypeOption.value === 'users') {
    label = `What is ${userCategory} type Users?`;
    Info = (
      <Text py={3}>
        Teleport users. SSO (Single Sign-On) users are ephemeral and may not
        exist in the list of users. You can create a SSO username by typing in
        the search bar above and pressing enter. <br />
        <br />
        Learn more about{' '}
        <ExternalLink
          target="_blank"
          href="https://goteleport.com/docs/zero-trust-access/sso/"
        >
          SSO
        </ExternalLink>
        .
      </Text>
    );
  }

  if (userTypeOption.value === 'access-lists') {
    label = `What is ${userCategory} type Access Lists?`;
    Info = <NestedAccessListInfo userCategory={userCategory} okta={okta} />;
  }

  if (userTypeOption.value === 'okta-access-lists') {
    label = 'What are Okta Access Lists?';
    Info = (
      <Stack gap={2} pb={3} pt={1}>
        <Text>
          Access Lists created through the Teleport Okta integration by{' '}
          <ExternalLink
            target="_blank"
            href="https://goteleport.com/docs/admin-guides/access-controls/okta/app-and-group-sync/#how-it-works"
          >
            synchronizing from Okta
          </ExternalLink>{' '}
          groups and applications with assignments.{' '}
        </Text>
        <NestedAccessListInfo userCategory={userCategory} />
      </Stack>
    );
  }

  return (
    <CollapsibleInfoSection
      size="large"
      openLabel={label}
      closeLabel={label}
      defaultOpen={false}
    >
      {Info}
    </CollapsibleInfoSection>
  );
}

const nestedAccessListDoc =
  'https://goteleport.com/docs/identity-governance/access-lists/nested-access-lists/';

function NestedAccessListInfo({
  userCategory,
  okta,
}: {
  userCategory: UserCategory;
  okta?: Okta;
}) {
  let oktaInfo;
  if (okta && !okta.hasPlugin) {
    oktaInfo = (
      <Alert
        kind="outline-info"
        mt={4}
        wrapContents
        primaryAction={{
          content: 'Enroll Okta Integration',
          linkTo: cfg.getIntegrationEnrollRoute('okta'),
        }}
        details={
          <>
            Enrolling Okta integration imports Okta applications and groups as
            Teleport Access Lists (optional opt-in). Those access lists can then
            also be assigned as {userCategory}
            s.
            <br />
            <ExternalLink
              target="_blank"
              href="https://goteleport.com/docs/identity-governance/integrations/okta/"
            >
              Learn more.
            </ExternalLink>
          </>
        }
      >
        Did you know?
      </Alert>
    );
  } else if (okta && !okta.hasAppGroupSyncEnabled) {
    oktaInfo = (
      <Info
        mt={4}
        wrapContents
        primaryAction={{
          content: 'Enable Okta Apps and Groups Sync',
          linkTo: cfg.getIntegrationEnrollRoute(
            'okta',
            OktaIntegrationStepType.AppGroupSync
          ),
        }}
        details={
          <>
            Enabling <Mark>application and group sync</Mark> in your Okta
            integration will sync Okta apps and groups as Access Lists which
            then can also be assigned as {getPluralUserCategory(userCategory)}.
            <br />
            <ExternalLink
              target="_blank"
              href="https://goteleport.com/docs/identity-governance/integrations/okta/app-and-group-sync/"
            >
              Learn more.
            </ExternalLink>
          </>
        }
      >
        Did you know?
      </Info>
    );
  }

  if (userCategory === 'owner') {
    return (
      <Box>
        <Text>
          An access list can be added as a owner. Members of the added access
          list will inherit the owner permissions from this new access list.
          Learn more about{' '}
          <ExternalLink target="_blank" href={nestedAccessListDoc}>
            nested access lists
          </ExternalLink>
          .
        </Text>
        {oktaInfo}
      </Box>
    );
  }

  if (userCategory === 'member') {
    return (
      <Box>
        <Text>
          An access list can be added as a member. Members of the added access
          list will inherit the member permissions from this new access list.
          Learn more about{' '}
          <ExternalLink target="_blank" href={nestedAccessListDoc}>
            nested access lists
          </ExternalLink>
          .
        </Text>
        {oktaInfo}
      </Box>
    );
  }
}

function getPluralUserCategory(user: UserCategory) {
  switch (user) {
    case 'member':
      return 'members';
    case 'owner':
      return 'owners';
    default:
      user satisfies never;
  }
}
