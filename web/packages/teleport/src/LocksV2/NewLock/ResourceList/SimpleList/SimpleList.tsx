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

import React, { useState, useEffect, useMemo } from 'react';

import useTeleport from 'teleport/useTeleport';
import { User } from 'teleport/services/user';
import { MfaDevice } from 'teleport/services/mfa';

import { TableWrapper, SimpleListProps } from '../common';
import { CommonListProps, LockResourceKind } from '../../common';

import Users from './Users';
import { MfaDevices } from './MfaDevices';

export type SimpleListOpts = {
  getFetchFn(
    selectedResourceKind: LockResourceKind
  ): (p: any, signal?: AbortSignal) => Promise<any>;
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
