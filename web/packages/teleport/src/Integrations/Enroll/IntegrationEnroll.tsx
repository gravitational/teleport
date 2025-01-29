/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import { Flex } from 'design';

import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import { Route, Switch } from 'teleport/components/Router';
import cfg from 'teleport/config';

import { NoCodeIntegrationDescription } from './common';
import { getRoutesToEnrollIntegrations } from './IntegrationRoute';
import { IntegrationTiles } from './IntegrationTiles';
import { MachineIDIntegrationSection } from './MachineIDIntegrationSection';

export function IntegrationEnroll() {
  return (
    <FeatureBox>
      <Switch>
        {getRoutesToEnrollIntegrations()}
        <Route
          path={cfg.routes.integrationEnroll}
          component={IntegrationPicker}
        />
      </Switch>
    </FeatureBox>
  );
}

export function IntegrationPicker() {
  return (
    <>
      <FeatureHeader>
        <FeatureHeaderTitle>Select Integration Type</FeatureHeaderTitle>
      </FeatureHeader>
      <Flex flexDirection="column" gap={4}>
        <Flex flexDirection="column">
          <NoCodeIntegrationDescription />
          <Flex mb={2} gap={3} flexWrap="wrap">
            <IntegrationTiles />
          </Flex>
        </Flex>
        <Flex flexDirection="column">
          <MachineIDIntegrationSection />
        </Flex>
      </Flex>
    </>
  );
}
