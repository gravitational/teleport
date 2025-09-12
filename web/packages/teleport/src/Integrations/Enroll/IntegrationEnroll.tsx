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

import {
  BotIntegration,
  integrations as botIntegrations,
  BotTile,
} from 'teleport/Bots/Add/AddBotsPicker';
import { FeatureBox } from 'teleport/components/Layout';
import { Route, Switch } from 'teleport/components/Router';
import cfg from 'teleport/config';
import { useNoMinWidth } from 'teleport/Main';
import useTeleport from 'teleport/useTeleport';

import { getRoutesToEnrollIntegrations } from './IntegrationRoute';
import {
  installableIntegrations,
  IntegrationTileSpec,
} from './IntegrationTiles/integrations';
import {
  displayName,
  IntegrationTileWithSpec,
  IntegrationPicker as SharedIntegrationPicker,
} from './Shared';

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
  return displayName(a).localeCompare(displayName(b));
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

  const integrations = [...installableIntegrations(), ...botIntegrations];
  useNoMinWidth();

  const renderIntegration = (i: Integration) => {
    if (i.type === 'integration') {
      return (
        <IntegrationTileWithSpec
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
  };

  return (
    <SharedIntegrationPicker
      integrations={integrations}
      renderIntegration={renderIntegration}
      initialSort={initialSort}
      canCreate={true}
    />
  );
}
