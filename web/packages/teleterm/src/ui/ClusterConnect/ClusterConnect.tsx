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

import { useState } from 'react';

import Dialog from 'design/Dialog';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import { DialogClusterConnect } from 'teleterm/ui/services/modals';
import { RootClusterUri } from 'teleterm/ui/uri';

import { ClusterAdd } from './ClusterAdd';
import { ClusterLogin } from './ClusterLogin';
import { dialogCss } from './spacing';

export function ClusterConnect(props: {
  dialog: DialogClusterConnect;
  hidden?: boolean;
}) {
  const [createdClusterUri, setCreatedClusterUri] = useState<
    RootClusterUri | undefined
  >();
  const { clustersService } = useAppContext();
  const clusterUri = props.dialog.clusterUri || createdClusterUri;

  function handleClusterAdd(clusterUri: RootClusterUri): void {
    const cluster = clustersService.findCluster(clusterUri);
    if (cluster?.connected) {
      props.dialog.onSuccess(clusterUri);
    } else {
      setCreatedClusterUri(clusterUri);
    }
  }

  return (
    <Dialog
      dialogCss={dialogCss}
      disableEscapeKeyDown={false}
      onClose={props.dialog.onCancel}
      open={!props.hidden}
      keepInDOMAfterClose
    >
      {!clusterUri ? (
        <ClusterAdd
          onCancel={props.dialog.onCancel}
          onSuccess={handleClusterAdd}
          prefill={{ clusterAddress: props.dialog.prefill?.clusterAddress }}
        />
      ) : (
        <ClusterLogin
          reason={props.dialog.reason}
          clusterUri={clusterUri}
          prefill={{ username: props.dialog.prefill?.username }}
          onCancel={props.dialog.onCancel}
          onSuccess={() => props.dialog.onSuccess(clusterUri)}
        />
      )}
    </Dialog>
  );
}
