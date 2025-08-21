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

import { useMemo } from 'react';
import styled from 'styled-components';

import { Box, Flex } from 'design';
import * as Icons from 'design/Icon';

import {
  BotIntegration,
  integrations as botIntegrations,
  BotTile,
} from 'teleport/Bots/Add/AddBotsPicker';
import { useUrlFiltering } from 'teleport/components/hooks';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import { Route, Switch } from 'teleport/components/Router';
import cfg from 'teleport/config';
import { TextIcon } from 'teleport/Discover/Shared';
import { type IntegrationTag } from 'teleport/Integrations/Enroll/IntegrationTiles/integrations';
import { IntegrationCardWithSpec } from 'teleport/Integrations/Enroll/IntegrationTiles/IntegrationTiles';
import {
  filterIntegrations,
  titleOrName,
} from 'teleport/Integrations/Enroll/utils/filters';
import useTeleport from 'teleport/useTeleport';

import { getRoutesToEnrollIntegrations } from './IntegrationRoute';
import {
  installableIntegrations,
  integrationTagOptions,
  IntegrationTileSpec,
} from './IntegrationTiles/integrations';
import { Container, FilterPanel } from './Shared';

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

type Integration = IntegrationTileSpec | BotIntegration;

const isGuided = (i: Integration) => {
  if ('guided' in i) {
    return i.guided;
    // non-Bot Integrations are guided
  } else {
    return true;
  }
};

const sortByName = (a: Integration, b: Integration) => {
  return titleOrName(a).localeCompare(titleOrName(b));
};

const initialSort = (a: Integration, b: Integration) => {
  return (
    (isGuided(b) ? (isGuided(a) ? 0 : 1) : isGuided(a) ? -1 : 0) ||
    sortByName(a, b)
  );
};

export function IntegrationPicker() {
  const ctx = useTeleport();
  const hasCreateBotPermission = ctx.getFeatureFlags().addBots;
  const hasIntegrationAccess = ctx.storeUser.getIntegrationsAccess().create;
  const hasExternalAuditStorage =
    ctx.storeUser.getExternalAuditStorageAccess().create;

  const { params, setParams } = useUrlFiltering({});

  const integrations = [...installableIntegrations(), ...botIntegrations];

  const sortedIntegrations = useMemo(() => {
    const sorted = integrations.toSorted((a, b) => {
      // Prioritize guided tiles if no sort params
      if (!params.sort) {
        return initialSort(a, b);
      }

      // Otherwise sort by name
      return sortByName(a, b);
    });

    if (params.sort?.dir === 'DESC') {
      sorted.reverse();
    }

    return sorted;
  }, [integrations, params.sort]);

  const filteredIntegrations = useMemo(
    () =>
      filterIntegrations(
        sortedIntegrations,
        (params.kinds as IntegrationTag[]) || [],
        params.search || ''
      ),
    [params.kinds, sortedIntegrations, params.search]
  );

  return (
    <>
      <Box my={3}>
        <FeatureHeader>
          <FeatureHeaderTitle>Enroll a New Integration</FeatureHeaderTitle>
        </FeatureHeader>
      </Box>
      <Flex flexDirection="column" gap={4}>
        <FilterPanel
          params={params}
          setParams={setParams}
          integrationTagOptions={integrationTagOptions}
        />
        {!filteredIntegrations.length && (
          <TextIcon>
            <Icons.Magnifier size="small" /> No results found
          </TextIcon>
        )}
        <Box mb={4}>
          <Container role="grid">
            {filteredIntegrations.map(i => {
              if (i.type === 'integration') {
                return (
                  <IntegrationCardWithSpec
                    key={i.kind}
                    spec={i}
                    hasIntegrationAccess={hasIntegrationAccess}
                    hasExternalAuditStorage={hasExternalAuditStorage}
                  />
                );
              }

              if (i.type === 'bot') {
                return (
                  <BotTile
                    key={i.kind}
                    integration={i}
                    hasCreateBotPermission={hasCreateBotPermission}
                  />
                );
              }
            })}
          </Container>
        </Box>
      </Flex>
    </>
  );
}
