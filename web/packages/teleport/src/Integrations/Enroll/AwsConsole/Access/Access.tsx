import { useHistory, useParams } from 'react-router';

import Box from 'design/Box';
import { ButtonPrimary, ButtonSecondary } from 'design/Button';
import Flex from 'design/Flex';
import { H2, P2 } from 'design/Text';

import cfg from 'teleport/config';
import { ProfilesEmptyState } from 'teleport/Integrations/Enroll/AwsConsole/Access/ProfilesEmptyState';
import {
  Profile,
  ProfilesTable,
} from 'teleport/Integrations/Enroll/AwsConsole/Access/ProfilesTable';
import { IntegrationKind } from 'teleport/services/integrations';

export function Access() {
  const { name } = useParams<{ name: string }>();
  const history = useHistory();
  const profiles: Profile[] = [
    {
      name: 'Profile-A',
      tags: ['tagA: value', 'tagD: 1'],
      roles: ['roleC, roleA'],
    },
    {
      name: 'Profile-B',
      tags: ['tagA: value', 'tagD: 1'],
      roles: ['roleC, roleA'],
    },
    {
      name: 'Profile-C',
      tags: ['tagA: value', 'tagD: 1'],
      roles: ['roleC, roleA'],
    },
    {
      name: 'Profile-D',
      tags: ['tagA: value', 'tagD: 1'],
      roles: ['roleC, roleA'],
    },
    {
      name: 'Profile-E',
      tags: ['tagA: value', 'tagD: 1'],
      roles: ['roleC, roleA'],
    },
  ];

  return (
    <Box pt={3}>
      <H2>Configure Access</H2>
      <P2 mb={3}>
        Import and synchronize AWS IAM Roles Anywhere Profiles into Teleport.
        Imported Profiles will be available as Resources with each Role
        available as an account.
      </P2>
      {profiles ? (
        <ProfilesTable profiles={profiles} />
      ) : (
        <ProfilesEmptyState />
      )}
      <Flex gap={3} mt={3}>
        <ButtonPrimary width="200px" disabled={true}>
          Enable Sync
        </ButtonPrimary>
        <ButtonSecondary
          onClick={() =>
            history.push(
              cfg.getIntegrationEnrollChildRoute(
                IntegrationKind.AwsOidc,
                name,
                IntegrationKind.AwsConsole,
                'integration'
              )
            )
          }
          width="100px"
        >
          Back
        </ButtonSecondary>
      </Flex>
    </Box>
  );
}
