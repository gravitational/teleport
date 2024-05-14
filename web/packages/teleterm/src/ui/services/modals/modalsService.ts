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

import { useStore } from 'shared/libs/stores';

import * as types from 'teleterm/services/tshd/types';
import { RootClusterUri } from 'teleterm/ui/uri';
import { ResourceSearchError } from 'teleterm/ui/services/resources';

import { PromptMfaRequest } from 'teleterm/services/tshdEvents';

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
   * The old dialog is canceled, if possible.
   *
   * The returned closeDialog function can be used to close the dialog and automatically call the
   * dialog's onCancel callback (if present).
   */
  openRegularDialog(dialog: Dialog): { closeDialog: () => void } {
    this.state.regular['onCancel']?.();
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
   * The old dialog is canceled, if possible.
   *
   * The returned closeDialog function can be used to close the dialog and automatically call the
   * dialog's onCancel callback (if present).
   */
  openImportantDialog(dialog: Dialog): { closeDialog: () => void } {
    this.state.important['onCancel']?.();
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
  /**
   * Supplying clusterUri makes the modal go straight to the credentials step and skips the first
   * step with providing the cluster address.
   */
  clusterUri: RootClusterUri | undefined;
  reason: ClusterConnectReason | undefined;
  prefill: { clusterAddress: string; username: string } | undefined;
  onSuccess(clusterUri: RootClusterUri): void;
  onCancel: (() => void) | undefined;
}

export interface ClusterConnectReasonGatewayCertExpired {
  kind: 'reason.gateway-cert-expired';
  targetUri: uri.GatewayTargetUri;
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
  rootClusterUri: RootClusterUri;
  numberOfDocuments: number;
  onConfirm?(): void;
  onCancel?(): void;
}

export interface DialogDeviceTrustAuthorize {
  kind: 'device-trust-authorize';
  rootClusterUri: RootClusterUri;
  onAuthorize(): Promise<void>;
  onCancel(): void;
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

export interface DialogHeadlessAuthentication {
  kind: 'headless-authn';
  rootClusterUri: RootClusterUri;
  headlessAuthenticationId: string;
  headlessAuthenticationClientIp: string;
  skipConfirm: boolean;
  onSuccess(): void;
  onCancel(): void;
}

export interface DialogReAuthenticate {
  kind: 'reauthenticate';
  promptMfaRequest: PromptMfaRequest;
  onSuccess(totpCode: string): void;
  onCancel(): void;
}

export type Dialog =
  | DialogClusterConnect
  | DialogClusterLogout
  | DialogDocumentsReopen
  | DialogDeviceTrustAuthorize
  | DialogUsageData
  | DialogUserJobRole
  | DialogResourceSearchErrors
  | DialogHeadlessAuthentication
  | DialogReAuthenticate
  | DialogNone;
