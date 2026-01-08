import { Link as ExternalLink, Stack, Text } from 'design';
import { CollapsibleInfoSection } from 'design/CollapsibleInfoSection';

import { UserTypeOption } from '../../types';

type UserCategory = 'owner' | 'member';

export function CollapsibleAccessListTypeInfo({
  userTypeOption,
  userCategory,
}: {
  userTypeOption: UserTypeOption;
  userCategory: UserCategory;
}) {
  let Info;
  let label = '';

  if (userTypeOption.value === 'users') {
    label = 'What are Users?';
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
    label = 'What are Access Lists?';
    Info = <NestedAccessListInfo userCategory={userCategory} />;
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
}: {
  userCategory: UserCategory;
}) {
  if (userCategory === 'owner') {
    return (
      <Text>
        An access list can be added as a owner. Members of the added access list
        will inherit the owner permissions from this new access list. Learn more
        about{' '}
        <ExternalLink target="_blank" href={nestedAccessListDoc}>
          nested access lists
        </ExternalLink>
        .
      </Text>
    );
  }

  if (userCategory === 'member') {
    return (
      <Text>
        An access list can be added as a member. Members of the added access
        list will inherit the member permissions from this new access list.
        Learn more about{' '}
        <ExternalLink target="_blank" href={nestedAccessListDoc}>
          nested access lists
        </ExternalLink>
        .
      </Text>
    );
  }
}
