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
import { ResourceIcon } from 'design/ResourceIcon';

import { GuidedFlow, View } from '../Shared/GuidedFlow';
import { AddBotToWorkflow } from './AddBotToWorkflow';
import { ConfigureBot } from './ConfigureBot';
import { ConnectGitHub } from './ConnectGitHub';
import { Finish } from './Finish';
import { GitHubFlowProvider } from './useGitHubFlow';

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
        icon={<ResourceIcon name="github" width="20px" />}
        views={views}
        name="GitHub Actions"
      />
    </GitHubFlowProvider>
  );
}
