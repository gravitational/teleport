/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { FeatureBox } from 'teleport/components/Layout';
import { Route, Switch } from 'teleport/components/Router';
import cfg from 'teleport/config';

import { BotFlowType } from '../types';
import { AddBotsPicker } from './AddBotsPicker';
import GitHubActionsFlow from './GitHubActions';

export function AddBots() {
  return (
    <FeatureBox>
      <Switch>
        <Route
          path={cfg.getBotsNewRoute(BotFlowType.GitHubActions)}
          component={GitHubActionsFlow}
        />
        <Route path={cfg.getBotsNewRoute()} component={AddBotsPicker} />
      </Switch>
    </FeatureBox>
  );
}
