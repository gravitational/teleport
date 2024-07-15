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

import { GitHubIcon } from 'design/SVGIcon';

import { BotFlowType } from 'teleport/Bots/types';

import cfg from 'teleport/config';

import { IntegrationEnrollKind } from 'teleport/services/userEvent';

import { GuidedFlow, View } from '../Shared/GuidedFlow';

import { ConnectGitHub } from './ConnectGitHub';

import { ConfigureBot } from './ConfigureBot';
import { AddBotToWorkflow } from './AddBotToWorkflow';
import { Finish } from './Finish';
import { GitHubFlowProvider } from './useGitHubFlow';

export const GitHubActionsFlow = {
  title: 'GitHub Actions',
  link: cfg.getBotsNewRoute(BotFlowType.GitHubActions),
  icon: <GitHubIcon size={80} />,
  kind: IntegrationEnrollKind.MachineIDGitHubActions,
  guided: true,
};

const views: View[] = [
  {
    name: 'Configure Bot Access',
    component: ConfigureBot,
  },
  {
    name: 'Connect GitHub',
    component: ConnectGitHub,
  },
  {
    name: 'Add Bot to GitHub',
    component: AddBotToWorkflow,
  },
  {
    name: 'Finish',
    component: Finish,
  },
];

export function GitHubActions() {
  return (
    <GitHubFlowProvider>
      <GuidedFlow
        title="GitHub Actions and Machine ID Integration"
        icon={<GitHubIcon size={20} />}
        views={views}
        name={GitHubActionsFlow.title}
      />
    </GitHubFlowProvider>
  );
}
