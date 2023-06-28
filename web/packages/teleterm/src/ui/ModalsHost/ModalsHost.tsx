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

import { ClusterConnect } from 'teleterm/ui/ClusterConnect';
import { DocumentsReopen } from 'teleterm/ui/DocumentsReopen';
import { Dialog } from 'teleterm/ui/services/modals';

import { ClusterLogout } from '../ClusterLogout';
import { ResourceSearchErrors } from '../Search/ResourceSearchErrors';
import { assertUnreachable } from '../utils';

import { UsageData } from './modals/UsageData';
import { UserJobRole } from './modals/UserJobRole';

import { PromptWebauthn } from '../ClusterConnect/ClusterLogin/FormLogin/PromptWebauthn';

export default function ModalsHost() {
  const { modalsService } = useAppContext();
  const { regular: regularDialog, important: importantDialog } =
    modalsService.useState();

  const closeRegularDialog = () => modalsService.closeRegularDialog();
  const closeImportantDialog = () => modalsService.closeImportantDialog();

  return (
    <>
      {renderDialog(regularDialog, closeRegularDialog)}
      {renderDialog(importantDialog, closeImportantDialog)}
    </>
  );
}

function renderDialog(dialog: Dialog, handleClose: () => void) {
  switch (dialog.kind) {
    case 'cluster-connect': {
      return (
        <ClusterConnect
          clusterUri={dialog.clusterUri}
          reason={dialog.reason}
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
    case 'cluster-logout': {
      return (
        <ClusterLogout
          clusterUri={dialog.clusterUri}
          clusterTitle={dialog.clusterTitle}
          onClose={handleClose}
        />
      );
    }
    case 'documents-reopen': {
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
    case 'usage-data': {
      return (
        <UsageData
          onCancel={() => {
            handleClose();
            dialog.onCancel();
          }}
          onAllow={() => {
            handleClose();
            dialog.onAllow();
          }}
          onDecline={() => {
            handleClose();
            dialog.onDecline();
          }}
        />
      );
    }
    case 'user-job-role': {
      return (
        <UserJobRole
          onCancel={() => {
            handleClose();
            dialog.onCancel();
          }}
          onSend={jobRole => {
            handleClose();
            dialog.onSend(jobRole);
          }}
        />
      );
    }

    case 'resource-search-errors': {
      return (
        <ResourceSearchErrors
          errors={dialog.errors}
          getClusterName={dialog.getClusterName}
          onCancel={() => {
            handleClose();
            dialog.onCancel();
          }}
        />
      );
    }


    case 'prompt-webauthn': {
      return (
        <PromptWebauthn
          prompt="tap"
          onCancel={() => {
            handleClose();
            dialog.onCancel?.();
          }}
        />
      );
    }

    case 'none': {
      return null;
    }

    default: {
      return assertUnreachable(dialog);
    }
  }
}
