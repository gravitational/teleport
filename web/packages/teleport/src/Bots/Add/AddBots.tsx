import React from 'react';

import { Switch, Route } from 'teleport/components/Router';
import cfg from 'teleport/config';
import { BotType } from '../types';
import GitHubActionsFlow from './GitHubActions';
import { FeatureBox } from 'teleport/components/Layout';
import { AddBotsPicker } from './AddBotsPicker';

export function AddBots() {
  return (
    <FeatureBox>
      <Switch>
        <Route
          path={cfg.getBotsNewRoute(BotType.GitHubActions)}
          component={GitHubActionsFlow}
        />
        <Route
          path={cfg.getBotsNewRoute()}
          component={AddBotsPicker}
        />
      </Switch>
    </FeatureBox>
  )
}

