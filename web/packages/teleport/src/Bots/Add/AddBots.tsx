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
import { Redirect, useParams } from 'teleport/components/Router';
import cfg from 'teleport/config';

import { BotFlowType } from '../types';
import { GitHubActionsK8s } from './GitHubActionsK8s/GitHubActionsK8s';
import GitHubActionsSshFlow from './GitHubActionsSsh';

export function AddBots() {
  const { type } = useParams<{ type?: string }>();

  if (type === BotFlowType.GitHubActionsSsh) {
    return (
      <FeatureBox>
        <GitHubActionsSshFlow />
      </FeatureBox>
    );
  }

  if (type === BotFlowType.GitHubActionsK8s) {
    return (
      <FeatureBox>
        <GitHubActionsK8s />
      </FeatureBox>
    );
  }

  return <Redirect to={`${cfg.getIntegrationEnrollRoute()}?tags=bot`} />;
}
