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

import * as types from 'teleterm/services/tshd/types';
import { RootClusterUri } from 'teleterm/ui/uri';
import { ResourceSearchError } from 'teleterm/ui/services/resources';

import { ImmutableStore } from '../immutableStore';

import type * as uri from 'teleterm/ui/uri';

type State = {
  // At most two modals can be displayed at the same time.
  // The important dialog is displayed above the regular one. This is to avoid losing the state of
  // the regular modal if we happen to need to interrupt whatever the user is doing and display an
  // important modal.
  important: Dialog;
  regular: Dialog;
};

export class ModalsService extends ImmutableStore<State> {
  state: State = {
    important: {
      kind: 'none',
    },
    regular: {
      kind: 'none',
    },
  };

  /**
   * openRegularDialog opens the given dialog as a regular dialog. A regular dialog can get covered
   * by an important dialog. The regular dialog won't get unmounted if an important dialog is shown
   * over the regular one.
   *
   * Calling openRegularDialog while another regular dialog is displayed will simply overwrite the
   * old dialog with the new one.
   *
   * The returned closeDialog function can be used to close the dialog and automatically call the
   * dialog's onCancel callback (if present).
   */
  openRegularDialog(dialog: Dialog): { closeDialog: () => void } {
    this.setState(draftState => {
      draftState.regular = dialog;
    });

    return {
      closeDialog: () => {
        this.closeRegularDialog();
        dialog['onCancel']?.();
      },
    };
  }

  /**
   * openImportantDialog opens the given dialog as an important dialog. An important dialog will be
   * displayed above a regular dialog but it will not affect the regular dialog in any other way.
   *
   * openImportantDialog should be reserved for situations where the interaction with the app
   * happens outside of its UI and requires us to interrupt the user and show them a modal.
   * One example of such scenario is showing the modal to relogin after the user attempts to make a
   * database connection through a gateway with expired user and db certs.
   *
   * Calling openImportantDialog while another important dialog is displayed will simply overwrite
   * the old dialog with the new one.
   *
   * The returned closeDialog function can be used to close the dialog and automatically call the
   * dialog's onCancel callback (if present).
   */
  openImportantDialog(dialog: Dialog): { closeDialog: () => void } {
    this.setState(draftState => {
      draftState.important = dialog;
    });

    return {
      closeDialog: () => {
        this.closeImportantDialog();
        dialog['onCancel']?.();
      },
    };
  }

  // TODO(ravicious): Remove this method in favor of calling openRegularDialog directly.
  openClusterConnectDialog(options: {
    clusterUri?: RootClusterUri;
    onSuccess?(clusterUri: RootClusterUri): void;
    onCancel?(): void;
  }) {
    return this.openRegularDialog({
      kind: 'cluster-connect',
      ...options,
    });
  }

  // TODO(ravicious): Remove this method in favor of calling openRegularDialog directly.
  openDocumentsReopenDialog(options: {
    onConfirm?(): void;
    onCancel?(): void;
  }) {
    return this.openRegularDialog({
      kind: 'documents-reopen',
      ...options,
    });
  }

  closeRegularDialog() {
    this.setState(draftState => {
      draftState.regular = {
        kind: 'none',
      };
    });
  }

  closeImportantDialog() {
    this.setState(draftState => {
      draftState.important = {
        kind: 'none',
      };
    });
  }

  useState() {
    return useStore(this).state;
  }
}

export interface DialogNone {
  kind: 'none';
}

export interface DialogClusterConnect {
  kind: 'cluster-connect';
  clusterUri?: RootClusterUri;
  reason?: ClusterConnectReason;
  onSuccess?(clusterUri: RootClusterUri): void;
  onCancel?(): void;
}

export interface ClusterConnectReasonGatewayCertExpired {
  kind: 'reason.gateway-cert-expired';
  targetUri: string;
  // The original RPC message passes gatewayUri but we might not always be able to resolve it to a
  // gateway, hence the use of undefined.
  gateway: types.Gateway | undefined;
}

export type ClusterConnectReason = ClusterConnectReasonGatewayCertExpired;

export interface DialogClusterLogout {
  kind: 'cluster-logout';
  clusterUri: RootClusterUri;
  clusterTitle: string;
}

export interface DialogDocumentsReopen {
  kind: 'documents-reopen';
  onConfirm?(): void;
  onCancel?(): void;
}

export interface DialogUsageData {
  kind: 'usage-data';
  onAllow(): void;
  onDecline(): void;
  onCancel(): void;
}

export interface DialogUserJobRole {
  kind: 'user-job-role';
  onSend(jobRole: string): void;
  onCancel(): void;
}

export interface DialogResourceSearchErrors {
  kind: 'resource-search-errors';
  errors: ResourceSearchError[];
  getClusterName: (resourceUri: uri.ClusterOrResourceUri) => string;
  onCancel: () => void;
}

export interface DialogPromptWebauthn {
  kind: 'prompt-webauthn';
  onSuccess?: () => void;
  onCancel: () => void;
}

export type Dialog =
  | DialogClusterConnect
  | DialogClusterLogout
  | DialogDocumentsReopen
  | DialogUsageData
  | DialogUserJobRole
  | DialogResourceSearchErrors
  | DialogPromptWebauthn
  | DialogNone;
