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

import { ButtonBorder, ButtonPrimary } from 'design/Button';
import { CardTile } from 'design/CardTile';
import Flex from 'design/Flex';
import { AmazonAws } from 'design/Icon';
import { H3, P2 } from 'design/Text';

import { rolesAnywhereCreateProfile } from 'teleport/Integrations/Enroll/awsLinks';

export function ProfilesEmptyState() {
  return (
    <CardTile alignItems="center" gap={4}>
      <AmazonAws />
      {/*todo mberg add Company: AWS IAM Identity-and-Access-Management icon*/}
      <Flex flexDirection="column" alignItems="center">
        <H3 mb={1}>No AWS IAM Roles Anywhere Profiles Found</H3>
        <P2>Create AWS IAM Roles Anywhere Profiles in your AWS console</P2>
      </Flex>
      <Flex gap={3}>
        <ButtonPrimary as="a" target="blank" href={rolesAnywhereCreateProfile}>
          Create AWS Roles Anywhere Profiles
        </ButtonPrimary>
        <ButtonBorder intent="primary">
          Refresh AWS Roles Anywhere Profiles
        </ButtonBorder>
      </Flex>
    </CardTile>
  );
}
