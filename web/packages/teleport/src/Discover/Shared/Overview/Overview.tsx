/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
  ActionButtons,
  Header,
  HeaderSubtitle,
} from 'teleport/Discover/Shared';
import { useDiscover } from 'teleport/Discover/useDiscover';

export function Overview() {
  const { prevStep, nextStep, isUpdateFlow } = useDiscover();

  const pageItems = {
    kubernetes: {
      overview: [
        `This guide uses Helm to install the Teleport agent into a cluster,
        and by default turns on auto-discovery of all apps in the cluster.`,
      ],
      prerequisites: [
        'Egress from your Kubernetes cluster to Teleport.',
        'Helm installed on your local machine.',
        'Kubernetes API access to install the Helm chart.',
      ],
    },
  };

  return (
    <>
      <Header>Overview</Header>
      <ul>
        {pageItems.kubernetes.overview.map((item, index) => (
          <li key={index}>{item}</li>
        ))}
      </ul>
      <Header>Prerequisites</Header>
      <HeaderSubtitle>
        Make sure you have these ready before continuing.
      </HeaderSubtitle>
      <ul>
        {pageItems.kubernetes.prerequisites.map((item, index) => (
          <li key={index}>{item}</li>
        ))}
      </ul>
      <ActionButtons
        onProceed={nextStep}
        onPrev={isUpdateFlow ? null : prevStep}
      />
    </>
  );
}
