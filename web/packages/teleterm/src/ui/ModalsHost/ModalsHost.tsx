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

import React from 'react';

import { useAppContext } from 'teleterm/ui/appContextProvider';

import { ClusterConnect } from 'teleterm/ui/ClusterConnect';
import { DocumentsReopen } from 'teleterm/ui/DocumentsReopen';
import { Dialog } from 'teleterm/ui/services/modals';
import { HeadlessAuthentication } from 'teleterm/ui/HeadlessAuthn';

import { ClusterLogout } from '../ClusterLogout';
import { ResourceSearchErrors } from '../Search/ResourceSearchErrors';
import { assertUnreachable } from '../utils';

import { UsageData } from './modals/UsageData';
import { UserJobRole } from './modals/UserJobRole';
import { ReAuthenticate } from './modals/ReAuthenticate';

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
          dialog={{
            ...dialog,
            onCancel: () => {
              handleClose();
              dialog.onCancel?.();
            },
            onSuccess: clusterUri => {
              handleClose();
              dialog.onSuccess(clusterUri);
            },
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
          rootClusterUri={dialog.rootClusterUri}
          numberOfDocuments={dialog.numberOfDocuments}
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

    case 'headless-authn': {
      return (
        <HeadlessAuthentication
          rootClusterUri={dialog.rootClusterUri}
          headlessAuthenticationId={dialog.headlessAuthenticationId}
          clientIp={dialog.headlessAuthenticationClientIp}
          skipConfirm={dialog.skipConfirm}
          onCancel={() => {
            handleClose();
            dialog.onCancel();
          }}
          onSuccess={() => {
            handleClose();
            dialog.onSuccess();
          }}
        />
      );
    }

    case 'reauthenticate': {
      return (
        <ReAuthenticate
          promptMfaRequest={dialog.promptMfaRequest}
          onSuccess={totpCode => {
            handleClose();
            dialog.onSuccess(totpCode);
          }}
          onCancel={() => {
            handleClose();
            dialog.onCancel();
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
