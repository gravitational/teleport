/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React, { useState, useEffect, useMemo } from 'react';

import useTeleport from 'teleport/useTeleport';
import { User } from 'teleport/services/user';
import { MfaDevice } from 'teleport/services/mfa';
import { KindRole, Resource } from 'teleport/services/resources';

import { TableWrapper, SimpleListProps } from '../common';
import { CommonListProps, LockResourceKind } from '../../common';

import { Roles } from './Roles';
import Users from './Users';
import { MfaDevices } from './MfaDevices';

export type SimpleListOpts = {
  getFetchFn(selectedResourceKind: LockResourceKind): (p: any) => Promise<any>;
  getTable(
    selectedResourceKind: LockResourceKind,
    resources: any[],
    listProps: SimpleListProps
  ): React.ReactElement;
};

export function SimpleList(props: CommonListProps & { opts: SimpleListOpts }) {
  const ctx = useTeleport();
  const [resources, setResources] = useState([]);

  useEffect(() => {
    let fetchFn;
    switch (props.selectedResourceKind) {
      case 'role':
        fetchFn = ctx.resourceService.fetchRoles;
        break;
      case 'user':
        fetchFn = ctx.userService.fetchUsers;
        break;
      case 'mfa_device':
        fetchFn = ctx.mfaService.fetchDevices;
        break;
      default:
        fetchFn = props.opts?.getFetchFn(props.selectedResourceKind);
        if (!fetchFn) {
          console.error(
            `[SimpleList.tsx] fetchFn not defined for resource kind ${props.selectedResourceKind}`
          );
          return; // don't do anything on error
        }
    }

    setResources([]);
    props.setAttempt({ status: 'processing' });
    fetchFn()
      .then(res => {
        setResources(res);
        props.setAttempt({ status: 'success' });
      })
      .catch((err: Error) => {
        props.setAttempt({ status: 'failed', statusText: err.message });
      });
  }, [props.selectedResourceKind]);

  const table = useMemo(() => {
    const listProps: SimpleListProps = {
      pageSize: props.pageSize,
      fetchStatus: props.attempt.status === 'processing' ? 'loading' : '',
      selectedResources: props.selectedResources,
      toggleSelectResource: props.toggleSelectResource,
    };
    switch (props.selectedResourceKind) {
      case 'role':
        return (
          <Roles roles={resources as Resource<KindRole>[]} {...listProps} />
        );
      case 'user':
        return <Users users={resources as User[]} {...listProps} />;
      case 'mfa_device':
        return (
          <MfaDevices mfaDevices={resources as MfaDevice[]} {...listProps} />
        );
      default:
        const table = props.opts?.getTable(
          props.selectedResourceKind,
          resources,
          listProps
        );
        if (table) {
          return table;
        }
        console.error(
          `[SimpleList.tsx] table not defined for resource kind ${props.selectedResourceKind}`
        );
    }
  }, [props.attempt, resources, props.selectedResources]);

  return (
    <TableWrapper
      className={props.attempt.status === 'processing' ? 'disabled' : ''}
    >
      {table}
    </TableWrapper>
  );
}
