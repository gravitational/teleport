/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import { useEffect, useState, useMemo } from 'react';

import useTeleport from 'teleport/useTeleport';

import type { AllowedTargets, LockTarget, TableData } from './types';

export const lockTargets: LockTarget[] = [
  { label: 'User', value: 'user' },
  { label: 'Role', value: 'role' },
  { label: 'Login', value: 'login' },
  { label: 'Node', value: 'node' },
  { label: 'MFA Device', value: 'mfa_device' },
  { label: 'Windows Desktop', value: 'windows_desktop' },
  // Skipped for now because it's not fully implemented.
  // { label: 'Device', value: 'device' },
];

export type UseGetTargetData = (
  targetType: AllowedTargets,
  clusterId: string,
  additionalTargets?: AdditionalTargets
) => TableData[];

export type AdditionalTargets = {
  [key: string]: {
    fetch: (options: any) => Promise<any>;
    handler: (setter: (data: TableData[]) => void, data: any) => void;
    options: any;
  };
};

export const useGetTargetData: UseGetTargetData = (
  targetType,
  clusterId,
  additionalTargets
) => {
  const [targetData, setTargetData] = useState<TableData[]>();
  const {
    desktopService: { fetchDesktops },
    mfaService: { fetchDevices },
    nodeService: { fetchNodes },
    resourceService: { fetchRoles },
    userService: { fetchUsers },
  } = useTeleport();

  const targetDataFilters = useMemo(() => {
    return {
      user: {
        fetch: fetchUsers,
        handler: (setter, users) => {
          const filteredData = users.map(u => ({
            name: u.name,
            roles: u.roles.join(', '),
          }));
          setter(filteredData);
        },
      },
      role: {
        fetch: fetchRoles,
        handler: (setter, roles) => {
          const filteredData = roles.map(r => ({
            name: r.name,
          }));
          setter(filteredData);
        },
      },
      node: {
        fetch: fetchNodes,
        handler: (setter, nodes) => {
          const filteredData = nodes.agents.map(n => ({
            name: n.hostname,
            addr: n.addr,
            labels: n.labels,
          }));
          setter(filteredData);
        },
        options: [
          clusterId,
          {
            limit: 10,
          },
        ],
      },
      mfa_device: {
        fetch: fetchDevices,
        handler: (setter, mfas) => {
          const filteredData = mfas.map(m => ({
            name: m.name,
            id: m.id,
            description: m.description,
            lastUsed: m.lastUsedDate.toUTCString(),
          }));
          setter(filteredData);
        },
      },
      windows_desktop: {
        fetch: fetchDesktops,
        handler: (setter, desktops) => {
          const filteredData = desktops.agents.map(d => ({
            name: d.name,
            addr: d.addr,
            labels: d.labels,
          }));
          setter(filteredData);
        },
        options: [clusterId, { limit: 10 }],
      },
    };
  }, [
    clusterId,
    fetchDesktops,
    fetchDevices,
    fetchNodes,
    fetchRoles,
    fetchUsers,
  ]);

  useEffect(() => {
    let action =
      targetDataFilters[targetType] || additionalTargets?.[targetType];
    if (!action) {
      // eslint-disable-next-line no-console
      console.log(`unknown target type ${targetType}`);
      setTargetData([]);
      return;
    }

    action.fetch
      .apply(null, action.options)
      .then(action.handler.bind(null, setTargetData));
  }, [additionalTargets, targetDataFilters, targetType]);

  return targetData;
};
