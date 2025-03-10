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

import { Fragment } from 'react';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import { ClusterConnect } from 'teleterm/ui/ClusterConnect';
import { DocumentsReopen } from 'teleterm/ui/DocumentsReopen';
import { HeadlessAuthentication } from 'teleterm/ui/HeadlessAuthn';
import { Dialog } from 'teleterm/ui/services/modals';

import { ClusterLogout } from '../ClusterLogout';
import { ResourceSearchErrors } from '../Search/ResourceSearchErrors';
import { assertUnreachable } from '../utils';
import { ChangeAccessRequestKind } from './modals/ChangeAccessRequestKind';
import { AskPin, ChangePin, OverwriteSlot, Touch } from './modals/HardwareKeys';
import { ReAuthenticate } from './modals/ReAuthenticate';
import { UsageData } from './modals/UsageData';
import { UserJobRole } from './modals/UserJobRole';

export default function ModalsHost() {
  const { modalsService } = useAppContext();
  const { regular: regularDialog, important: importantDialogs } =
    modalsService.useState();

  return (
    <>
      {regularDialog &&
        renderDialog({
          dialog: regularDialog.dialog,
          handleClose: regularDialog.close,
          hidden: !!importantDialogs.length,
        })}
      {importantDialogs.map(({ dialog, id, close }, index) => {
        const isLast = index === importantDialogs.length - 1;
        return (
          <Fragment key={id}>
            {renderDialog({
              dialog: dialog,
              handleClose: close,
              hidden: !isLast,
            })}
          </Fragment>
        );
      })}
    </>
  );
}

/**
 * Renders a dialog.
 * Each dialog must implement a `hidden` prop which visually hides the dialog
 * without unmounting it.
 * This is needed because tshd may want to display more than one dialog.
 * Also, we hide a regular dialog, when an important one is visible.
 */
function renderDialog({
  dialog,
  handleClose,
  hidden,
}: {
  dialog: Dialog;
  handleClose: () => void;
  hidden: boolean;
}) {
  if (!dialog) {
    return null;
  }

  switch (dialog.kind) {
    case 'cluster-connect': {
      return (
        <ClusterConnect
          hidden={hidden}
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
          hidden={hidden}
          clusterUri={dialog.clusterUri}
          clusterTitle={dialog.clusterTitle}
          onClose={handleClose}
        />
      );
    }
    case 'documents-reopen': {
      return (
        <DocumentsReopen
          hidden={hidden}
          rootClusterUri={dialog.rootClusterUri}
          numberOfDocuments={dialog.numberOfDocuments}
          onDiscard={() => {
            handleClose();
            dialog.onDiscard();
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
          hidden={hidden}
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
          hidden={hidden}
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
          hidden={hidden}
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
          hidden={hidden}
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
          hidden={hidden}
          promptMfaRequest={dialog.promptMfaRequest}
          onOtpSubmit={totpCode => {
            handleClose();
            dialog.onSuccess(totpCode);
          }}
          // This function needs to be stable between renders.
          onSsoContinue={dialog.onSsoContinue}
          onCancel={() => {
            handleClose();
            dialog.onCancel();
          }}
        />
      );
    }
    case 'change-access-request-kind': {
      return (
        <ChangeAccessRequestKind
          hidden={hidden}
          onConfirm={() => {
            handleClose();
            dialog.onConfirm();
          }}
          onCancel={() => {
            handleClose();
            dialog.onCancel();
          }}
        />
      );
    }
    case 'hardware-key-pin': {
      return (
        <AskPin
          hidden={hidden}
          req={dialog.req}
          onSuccess={res => {
            handleClose();
            dialog.onSuccess(res);
          }}
          onCancel={() => {
            handleClose();
            dialog.onCancel();
          }}
        />
      );
    }
    case 'hardware-key-touch': {
      return (
        <Touch
          hidden={hidden}
          req={dialog.req}
          onCancel={() => {
            handleClose();
            dialog.onCancel();
          }}
        />
      );
    }
    case 'hardware-key-pin-change': {
      return (
        <ChangePin
          hidden={hidden}
          req={dialog.req}
          onSuccess={dialog.onSuccess}
          onCancel={() => {
            handleClose();
            dialog.onCancel();
          }}
        />
      );
    }
    case 'hardware-key-slot-overwrite': {
      return (
        <OverwriteSlot
          hidden={hidden}
          req={dialog.req}
          onConfirm={dialog.onConfirm}
          onCancel={() => {
            handleClose();
            dialog.onCancel();
          }}
        />
      );
    }

    default: {
      return assertUnreachable(dialog);
    }
  }
}
