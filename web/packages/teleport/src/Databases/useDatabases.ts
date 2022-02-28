/*
Copyright 2021 Gravitational, Inc.

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

import { useState, useEffect } from 'react';
import useAttempt from 'shared/hooks/useAttemptNext';
import Ctx from 'teleport/teleportContext';
import useStickyClusterId from 'teleport/useStickyClusterId';
import { Database } from 'teleport/services/databases';

export default function useDatabases(ctx: Ctx) {
  const { attempt, run, setAttempt } = useAttempt('processing');
  const { clusterId, isLeafCluster } = useStickyClusterId();
  const username = ctx.storeUser.state.username;
  const canCreate = ctx.storeUser.getTokenAccess().create;
  const isEnterprise = ctx.isEnterprise;
  const version = ctx.storeUser.state.cluster.authVersion;
  const authType = ctx.storeUser.state.authType;

  const [databases, setDatabases] = useState<Database[]>([]);
  const [isAddDialogVisible, setIsAddDialogVisible] = useState(false);

  useEffect(() => {
    run(() =>
      ctx.databaseService
        .fetchDatabases(clusterId)
        .then(res => setDatabases(res.databases))
    );
  }, [clusterId]);

  const fetchDatabases = () => {
    return ctx.databaseService
      .fetchDatabases(clusterId)
      .then(res => setDatabases(res.databases))
      .catch((err: Error) =>
        setAttempt({ status: 'failed', statusText: err.message })
      );
  };

  const hideAddDialog = () => {
    setIsAddDialogVisible(false);
    fetchDatabases();
  };

  const showAddDialog = () => {
    setIsAddDialogVisible(true);
  };

  return {
    databases,
    attempt,
    canCreate,
    isLeafCluster,
    isEnterprise,
    hideAddDialog,
    showAddDialog,
    isAddDialogVisible,
    username,
    version,
    clusterId,
    authType,
  };
}

export type State = ReturnType<typeof useDatabases>;
