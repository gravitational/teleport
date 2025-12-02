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

import { ButtonPrimary, ButtonSecondary } from 'design/Button';
import { ResourceIcon } from 'design/ResourceIcon';

import { useNoMinWidth } from 'teleport/Main';

import { FlowStepProps, GuidedFlow, View } from '../Shared/GuidedFlow';
import { ConfigureAccess } from './ConfigureAccess';
import { ConnectGitHub } from './ConnectGitHub';
import { GitHubK8sFlowProvider } from './useGitHubK8sFlow';

function Placeholder(props: FlowStepProps) {
  const { nextStep, prevStep } = props;
  return (
    <div>
      placeholder<ButtonPrimary onClick={nextStep}>Next</ButtonPrimary>
      <ButtonSecondary onClick={prevStep}>Back</ButtonSecondary>
    </div>
  );
}

const views: View[] = [
  {
    name: 'Welcome',
    component: Placeholder,
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
    name: 'Setup workflow',
    component: Placeholder,
  },
];

export function GitHubActionsK8s() {
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
