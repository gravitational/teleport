import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import { useHistory, useParams } from 'react-router';

import * as Alerts from 'design/Alert';
import Box from 'design/Box';
import { ButtonPrimary, ButtonSecondary } from 'design/Button';
import Flex from 'design/Flex';
import { H2, P2 } from 'design/Text';

import cfg from 'teleport/config';
import {
  Profile,
  Profiles,
} from 'teleport/Integrations/Enroll/AwsConsole/Access/Profiles';
import { ProfilesEmptyState } from 'teleport/Integrations/Enroll/AwsConsole/Access/ProfilesEmptyState';
import { ProfilesFilterOption } from 'teleport/Integrations/Enroll/AwsConsole/Access/ProfilesFilter';
import { IntegrationKind } from 'teleport/services/integrations';

const mockResponse: Profile[] = [
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

export function Access() {
  const { name } = useParams<{ name: string }>();
  const history = useHistory();
  const [syncAll, setSyncAll] = useState(true);
  const [filters, setFilters] = useState<ProfilesFilterOption[]>([]);

  const {
    status,
    error,
    refetch,
    data: profiles,
  } = useQuery({
    queryKey: ['profiles'],
    queryFn: (): Promise<Profile[]> => {
      return Promise.resolve(mockResponse);
    },
  });

  return (
    <Box pt={3}>
      <H2>Configure Access</H2>
      <P2 mb={3}>
        Import and synchronize AWS IAM Roles Anywhere Profiles into Teleport.
        Imported Profiles will be available as Resources with each Role
        available as an account.
      </P2>
      {status === 'error' && (
        <Alerts.Danger details={error.message}>
          Error loading profiles
        </Alerts.Danger>
      )}
      {status === 'success' && (!profiles || profiles.length === 0) && (
        <ProfilesEmptyState />
      )}
      {status === 'success' && profiles.length > 0 && (
        <Profiles
          profiles={profiles}
          refetch={refetch}
          syncAll={syncAll}
          setSyncAll={setSyncAll}
          filters={filters}
          setFilters={setFilters}
        />
      )}
      <Flex gap={3} mt={3}>
        <ButtonPrimary
          width="200px"
          disabled={!syncAll || filters.length !== 0}
        >
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
