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

import { Dispatch, SetStateAction } from 'react';

import { ButtonPrimary } from 'design/Button';
import { CardTile } from 'design/CardTile';
import { FetchingConfig } from 'design/DataTable/types';
import Flex from 'design/Flex';
import * as Icons from 'design/Icon';
import { H3, P1, P2 } from 'design/Text';
import { Toggle } from 'design/Toggle';
import Validation from 'shared/components/Validation';

import { rolesAnywhereCreateProfile } from 'teleport/Integrations/Enroll/awsLinks';
import { RolesAnywhereProfile } from 'teleport/services/integrations';

import { ProfilesFilter, ProfilesFilterOption } from './ProfilesFilter';
import { ProfilesTable } from './ProfilesTable';

export function Profiles({
  data,
  fetchStatus,
  filters,
  setFilters,
  onFetchNext,
  onFetchPrev,
  refetch,
  syncAll,
  setSyncAll,
}: {
  data: RolesAnywhereProfile[];
  filters: ProfilesFilterOption[];
  setFilters: Dispatch<SetStateAction<ProfilesFilterOption[]>>;
  refetch: () => void;
  syncAll: boolean;
  setSyncAll: Dispatch<SetStateAction<boolean>>;
} & Omit<FetchingConfig, 'onFetchMore'>) {
  return (
    <Validation>
      <CardTile backgroundColor="levels.elevated">
        <Flex justifyContent="space-between">
          <Flex flexDirection="column">
            <H3>Sync IAM Profiles with Teleport as Resources</H3>
            <P2>
              You will be able to see the imported profiles on the Resources
              Page
            </P2>
          </Flex>
          <Flex alignItems="center" gap={3}>
            <ButtonPrimary
              gap={2}
              fill="minimal"
              intent="neutral"
              size="small"
              onClick={refetch}
            >
              <Icons.Refresh size="small" />
              Refresh
            </ButtonPrimary>
            <ButtonPrimary
              gap={2}
              intent="neutral"
              size="small"
              as="a"
              target="blank"
              href={rolesAnywhereCreateProfile}
            >
              Create AWS Roles Anywhere Profiles
              <Icons.NewTab size="small" />
            </ButtonPrimary>
            <Flex gap={1}>
              <Toggle
                isToggled={syncAll}
                onToggle={() => {
                  setSyncAll(!syncAll);
                }}
                size="large"
              />
              <P1>Import All Profiles</P1>
            </Flex>
          </Flex>
        </Flex>
        {!syncAll && (
          <ProfilesFilter filters={filters} setFilters={setFilters} />
        )}
        <ProfilesTable
          data={data}
          fetchStatus={fetchStatus}
          onFetchNext={onFetchNext}
          onFetchPrev={onFetchPrev}
        />
      </CardTile>
    </Validation>
  );
}
