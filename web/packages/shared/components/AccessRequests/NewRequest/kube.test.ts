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

import { checkForUnsupportedKubeRequestModes } from './kube';

test('checkForUnsupportedKubeRequestModes: non failed status', () => {
  const {
    affectedKubeClusterName,
    unsupportedKubeRequestModes,
    requiresNamespaceSelect,
  } = checkForUnsupportedKubeRequestModes({ status: '' });

  expect(affectedKubeClusterName).toBeFalsy();
  expect(unsupportedKubeRequestModes).toBeFalsy();
  expect(requiresNamespaceSelect).toBeFalsy();
});

test('checkForUnsupportedKubeRequestModes: failed status with unsupported kinds', () => {
  const {
    affectedKubeClusterName,
    unsupportedKubeRequestModes,
    requiresNamespaceSelect,
  } = checkForUnsupportedKubeRequestModes({
    status: 'failed',
    statusText: `Your Teleport roles request_mode field restricts you from requesting kinds [kube_cluster] for Kubernetes cluster pumpkin-kube-cluster. Allowed kinds: [pod secret]`,
  });

  expect(affectedKubeClusterName).toEqual(`pumpkin-kube-cluster`);
  expect(unsupportedKubeRequestModes).toEqual('[pod secret]');
  expect(requiresNamespaceSelect).toBeFalsy();
});

test('checkForUnsupportedKubeRequestModes: failed status with supported namespace', () => {
  const {
    affectedKubeClusterName,
    unsupportedKubeRequestModes,
    requiresNamespaceSelect,
  } = checkForUnsupportedKubeRequestModes({
    status: 'failed',
    statusText: `Your Teleport roles request_mode field restricts you from requesting kinds [kube_cluster] for Kubernetes cluster pumpkin-kube-cluster. Allowed kinds: [pod secret namespace]`,
  });

  expect(affectedKubeClusterName).toEqual(`pumpkin-kube-cluster`);
  expect(unsupportedKubeRequestModes).toBeFalsy();
  expect(requiresNamespaceSelect).toBeTruthy();
});
