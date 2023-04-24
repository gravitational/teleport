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

import { useEffect, useMemo, useState } from 'react';

import useTeleport from 'teleport/useTeleport';

import type {
  AllowedTargetResource,
  LockTargetOption,
  TableData,
} from './types';

export const lockTargetDropdownOptions: LockTargetOption[] = [
  { label: 'User', value: 'user' },
  { label: 'Role', value: 'role' },
  { label: 'Login', value: 'login' },
  { label: 'Node', value: 'node' },
  { label: 'MFA Device', value: 'mfa_device' },
  { label: 'Windows Desktop', value: 'windows_desktop' },
  // Skipped for now because it's not fully implemented.
  // { label: 'Device', value: 'device' },
  // TODO(sshahcodes): add support for locking devices
];

export type UseGetTargetData = (
  targetType: AllowedTargetResource,
  clusterId: string,
  additionalTargets?: AdditionalTargets
) => TableData[];

export type AdditionalTargets = Partial<
  Record<AllowedTargetResource, { fetchData(): Promise<TableData[]> }>
>;

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

  const targetDataFilters = useMemo<
    Partial<
      Record<AllowedTargetResource, { fetchData(): Promise<TableData[]> }>
    >
  >(() => {
    return {
      user: {
        fetchData: async () => {
          const users = await fetchUsers();
          return users.map(u => ({
            name: u.name,
            roles: u.roles.join(', '),
            targetValue: u.name,
          }));
        },
      },
      role: {
        fetchData: async () => {
          const roles = await fetchRoles();
          return roles.map(r => ({ name: r.name, targetValue: r.name }));
        },
      },
      node: {
        fetchData: async () => {
          const nodes = await fetchNodes(clusterId, { limit: 10 });
          return nodes.agents.map(n => ({
            name: n.hostname,
            addr: n.addr,
            labels: n.labels,
            targetValue: n.id,
          }));
        },
      },
      mfa_device: {
        fetchData: async () => {
          const mfas = await fetchDevices();
          return mfas.map(m => ({
            name: m.name,
            id: m.id,
            description: m.description,
            lastUsed: m.lastUsedDate.toUTCString(),
            targetValue: m.id,
          }));
        },
      },
      windows_desktop: {
        fetchData: async () => {
          const desktops = await fetchDesktops(clusterId, { limit: 10 });
          return desktops.agents.map(d => ({
            name: d.name,
            addr: d.addr,
            labels: d.labels,
            targetValue: d.name,
          }));
        },
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
      console.warn(`unknown target type ${targetType}`);
      setTargetData([]);
      return;
    }

    action.fetchData().then(targetData => {
      setTargetData(targetData);
    });
  }, [additionalTargets, targetDataFilters, targetType]);

  return targetData;
};
