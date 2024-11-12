/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import { Text } from 'design';

import { StyledBox } from 'teleport/Discover/Shared';

import { ShowConfigurationScript } from './ShowConfigurationScript';

export default {
  title: 'Teleport/Integrations/Shared/AwsOidc/ShowConfigurationScript',
};

export const Enabled = () => {
  return (
    <StyledBox width={700}>
      <ShowConfigurationScript scriptUrl="https://example.com?awsoidc-idp.sh" />
    </StyledBox>
  );
};

export const CustomDescription = () => {
  const description = <Text>Custom description</Text>;

  return (
    <StyledBox width={700}>
      <ShowConfigurationScript
        scriptUrl="https://example.com?awsoidc-idp.sh"
        description={description}
      />
    </StyledBox>
  );
};
