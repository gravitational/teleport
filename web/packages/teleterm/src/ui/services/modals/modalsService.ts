/*
Copyright 2019 Gravitational, Inc.

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

import { useStore } from 'shared/libs/stores';
import { ImmutableStore } from '../immutableStore';

type OpenProxyDbDialogOpts = {
  dbUri: string;
  port?: string;
  onSuccess?: (gatewayUri: string) => void;
};

export class ModalsService extends ImmutableStore<Dialog> {
  state: Dialog = {
    kind: 'none',
  };

  openDialog(dialog: Dialog) {
    this.setState(() => dialog);
  }

  openProxyDbDialog(opts: OpenProxyDbDialogOpts) {
    this.setState(() => ({
      kind: 'create-gateway',
      onSuccess: opts.onSuccess,
      targetUri: opts.dbUri,
      port: opts.port,
    }));
  }

  openClusterConnectDialog(
    clusterUri?: string,
    onSuccess?: (clusterUri: string) => void
  ) {
    this.setState(() => ({
      kind: 'cluster-connect',
      clusterUri,
      onSuccess,
    }));
  }

  openDocumentsReopenDialog(options: {
    onConfirm?(): void;
    onCancel?(): void;
  }) {
    this.setState(() => ({
      kind: 'documents-reopen',
      ...options,
    }));
  }

  closeDialog() {
    this.setState(() => ({
      kind: 'none',
    }));
  }

  useState() {
    return useStore(this).state;
  }
}

export interface DialogBase {
  kind: 'none';
}

export interface DialogNewGateway {
  kind: 'create-gateway';
  targetUri: string;
  port?: string;
  onSuccess?: (gatewayUri: string) => void;
}

export interface DialogClusterConnect {
  kind: 'cluster-connect';
  clusterUri?: string;
  onSuccess?(clusterUri: string): void;
}

export interface DialogClusterLogout {
  kind: 'cluster-logout';
  clusterUri: string;
  clusterTitle: string;
}

export interface DialogDocumentsReopen {
  kind: 'documents-reopen';

  onConfirm?(): void;

  onCancel?(): void;
}

export type Dialog =
  | DialogBase
  | DialogClusterConnect
  | DialogNewGateway
  | DialogClusterLogout
  | DialogDocumentsReopen;
