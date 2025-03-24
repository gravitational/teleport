/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import { generateCmd, GenerateCmdProps } from './HelmChart';

describe('generateCmd', () => {
  const baseParams: GenerateCmdProps = {
    namespace: 'teleport-agent',
    clusterName: 'EKS1',
    proxyAddr: 'proxyaddr.example.com:1234',
    tokenId: 'token-id',
    clusterVersion: '12.3.4',
    resourceId: 'resource-id',
    isEnterprise: false,
    isCloud: false,
    automaticUpgradesEnabled: false,
    automaticUpgradesTargetVersion: '',
  };
  const testCases: {
    name: string;
    params: GenerateCmdProps;
    expected: string;
  }[] = [
    {
      name: 'simplest case',
      params: { ...baseParams },
      expected: `cat << EOF > prod-cluster-values.yaml
roles: kube,app,discovery
authToken: token-id
proxyAddr: proxyaddr.example.com:1234
kubeClusterName: EKS1
labels:
    teleport.internal/resource-id: resource-id
EOF

helm install teleport-agent teleport/teleport-kube-agent -f prod-cluster-values.yaml --version 12.3.4 \\
--create-namespace --namespace teleport-agent`,
    },
    {
      name: 'disabled app discovery',
      params: { ...baseParams, disableAppDiscovery: true },
      expected: `cat << EOF > prod-cluster-values.yaml
roles: kube
authToken: token-id
proxyAddr: proxyaddr.example.com:1234
kubeClusterName: EKS1
labels:
    teleport.internal/resource-id: resource-id
EOF

helm install teleport-agent teleport/teleport-kube-agent -f prod-cluster-values.yaml --version 12.3.4 \\
--create-namespace --namespace teleport-agent`,
    },
    {
      name: 'join labels present',
      params: {
        ...baseParams,
        joinLabels: [
          { name: 'label1', value: 'value1' },
          { name: 'label2', value: 'value2' },
        ],
      },
      expected: `cat << EOF > prod-cluster-values.yaml
roles: kube,app,discovery
authToken: token-id
proxyAddr: proxyaddr.example.com:1234
kubeClusterName: EKS1
labels:
    teleport.internal/resource-id: resource-id
    label1: value1
    label2: value2
EOF

helm install teleport-agent teleport/teleport-kube-agent -f prod-cluster-values.yaml --version 12.3.4 \\
--create-namespace --namespace teleport-agent`,
    },
    {
      name: 'enterprise',
      params: {
        ...baseParams,
        isEnterprise: true,
      },
      expected: `cat << EOF > prod-cluster-values.yaml
roles: kube,app,discovery
authToken: token-id
proxyAddr: proxyaddr.example.com:1234
kubeClusterName: EKS1
labels:
    teleport.internal/resource-id: resource-id
enterprise: true
EOF

helm install teleport-agent teleport/teleport-kube-agent -f prod-cluster-values.yaml --version 12.3.4 \\
--create-namespace --namespace teleport-agent`,
    },
    {
      name: 'cloud automatic upgrades',
      params: {
        ...baseParams,
        isCloud: true,
        automaticUpgradesEnabled: true,
        automaticUpgradesTargetVersion: '14.5.6',
      },
      expected: `cat << EOF > prod-cluster-values.yaml
roles: kube,app,discovery
authToken: token-id
proxyAddr: proxyaddr.example.com:1234
kubeClusterName: EKS1
labels:
    teleport.internal/resource-id: resource-id
updater:
    enabled: true
    releaseChannel: "stable/cloud"
highAvailability:
    replicaCount: 2
    podDisruptionBudget:
        enabled: true
        minAvailable: 1
EOF

helm install teleport-agent teleport/teleport-kube-agent -f prod-cluster-values.yaml --version 14.5.6 \\
--create-namespace --namespace teleport-agent`,
    },
    {
      name: 'enterprise, with join labels, with cloud automatic upgrades',
      params: {
        ...baseParams,
        isEnterprise: true,
        isCloud: true,
        joinLabels: [
          { name: 'label1', value: 'value1' },
          { name: 'label2', value: 'value2' },
        ],
        automaticUpgradesEnabled: true,
        automaticUpgradesTargetVersion: '14.5.6',
      },
      expected: `cat << EOF > prod-cluster-values.yaml
roles: kube,app,discovery
authToken: token-id
proxyAddr: proxyaddr.example.com:1234
kubeClusterName: EKS1
labels:
    teleport.internal/resource-id: resource-id
    label1: value1
    label2: value2
enterprise: true
updater:
    enabled: true
    releaseChannel: "stable/cloud"
highAvailability:
    replicaCount: 2
    podDisruptionBudget:
        enabled: true
        minAvailable: 1
EOF

helm install teleport-agent teleport/teleport-kube-agent -f prod-cluster-values.yaml --version 14.5.6 \\
--create-namespace --namespace teleport-agent`,
    },
  ];

  test.each(testCases)('$name', testCase => {
    expect(generateCmd(testCase.params)).toEqual(testCase.expected);
  });
});
