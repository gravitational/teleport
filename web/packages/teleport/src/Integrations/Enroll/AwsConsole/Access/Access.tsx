/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { useQuery } from '@tanstack/react-query';
import { useEffect, useState } from 'react';
import { useHistory, useParams } from 'react-router';

import * as Alerts from 'design/Alert';
import Box from 'design/Box';
import { ButtonPrimary, ButtonSecondary } from 'design/Button';
import Flex from 'design/Flex';
import { Indicator } from 'design/Indicator';
import { H2, P2 } from 'design/Text';
import { InfoGuideButton } from 'shared/components/SlidingSidePanel/InfoGuide';

import cfg from 'teleport/config';
import {
  Profile,
  Profiles,
} from 'teleport/Integrations/Enroll/AwsConsole/Access/Profiles';
import { ProfilesEmptyState } from 'teleport/Integrations/Enroll/AwsConsole/Access/ProfilesEmptyState';
import { ProfilesFilterOption } from 'teleport/Integrations/Enroll/AwsConsole/Access/ProfilesFilter';
import { Guide } from 'teleport/Integrations/Enroll/AwsConsole/Guide';
import { IntegrationKind } from 'teleport/services/integrations';
import useTeleport from 'teleport/useTeleport';

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
  const ctx = useTeleport();
  const clusterId = ctx.storeUser.getClusterId();
  const resourceRoute = cfg.getUnifiedResourcesRoute(clusterId);

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
    queryKey: ['profiles', filters], // refetch when filters change
    queryFn: (): Promise<Profile[]> => {
      // todo mberg [blocked on API]
      return Promise.resolve(mockResponse);
    },
  });

  useEffect(() => {
    // clear filters on syncAll
    if (syncAll) {
      setFilters([]);
    }
  }, [syncAll]);

  return (
    <Box pt={3}>
      <Flex mt={3} justifyContent="space-between" alignItems="center">
        <H2>Configure Access</H2>
        <InfoGuideButton
          config={{
            guide: <Guide resourcesRoute={resourceRoute} />,
          }}
        />
      </Flex>
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
      {status === 'pending' && <Indicator />}
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
