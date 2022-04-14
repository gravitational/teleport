/**
 * Copyright 2021 Gravitational, Inc.
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

import React from 'react';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import ClusterLogout from '../ClusterLogout/ClusterLogout';
import { ClusterConnect } from 'teleterm/ui/ClusterConnect';
import { DocumentsReopen } from 'teleterm/ui/DocumentsReopen';

export default function ModalsHost() {
  const { modalsService } = useAppContext();
  const dialog = modalsService.useState();

  const handleClose = () => modalsService.closeDialog();

  if (dialog.kind === 'cluster-connect') {
    return (
      <ClusterConnect
        clusterUri={dialog.clusterUri}
        onCancel={() => {
          handleClose();
          dialog.onCancel?.();
        }}
        onSuccess={clusterUri => {
          handleClose();
          dialog.onSuccess(clusterUri);
        }}
      />
    );
  }

  if (dialog.kind === 'cluster-logout') {
    return (
      <ClusterLogout
        clusterUri={dialog.clusterUri}
        clusterTitle={dialog.clusterTitle}
        onClose={handleClose}
      />
    );
  }

  if (dialog.kind === 'documents-reopen') {
    return (
      <DocumentsReopen
        onCancel={() => {
          handleClose();
          dialog.onCancel();
        }}
        onConfirm={() => {
          handleClose();
          dialog.onConfirm();
        }}
      />
    );
  }

  return null;
}
