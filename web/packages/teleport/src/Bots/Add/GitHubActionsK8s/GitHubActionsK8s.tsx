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

import { ResourceIcon } from 'design/ResourceIcon';

import { useNoMinWidth } from 'teleport/Main';

import { GuidedFlow, View } from '../Shared/GuidedFlow';
import { TrackingProvider } from '../Shared/useTracking';
import { ConfigureAccess } from './ConfigureAccess';
import { ConnectGitHub } from './ConnectGitHub';
import { Finish } from './Finish';
import { GitHubK8sFlowProvider } from './useGitHubK8sFlow';
import { Welcome } from './Welcome';

const views: View[] = [
  {
    name: 'Welcome',
    component: Welcome,
  },
  {
    name: 'Connect GitHub',
    component: ConnectGitHub,
  },
  {
    name: 'Configure Access',
    component: ConfigureAccess,
  },
  {
    name: 'Set Up Workflow',
    component: Finish,
  },
];

export function GitHubActionsK8s() {
  useNoMinWidth();

  return (
    <TrackingProvider>
      <GitHubActionsK8sWithoutTracking />
    </TrackingProvider>
  );
}

export function GitHubActionsK8sWithoutTracking() {
  useNoMinWidth();

  return (
    <GitHubK8sFlowProvider>
      <GuidedFlow
        icon={<ResourceIcon name="github" width="20px" />}
        views={views}
        name="GitHub Actions + Kubernetes"
      />
    </GitHubK8sFlowProvider>
  );
}
