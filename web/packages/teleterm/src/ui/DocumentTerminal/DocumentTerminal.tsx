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

import React from 'react';
import {
  FileTransferActionBar,
  FileTransfer,
  FileTransferContextProvider,
} from 'shared/components/FileTransfer';

import Document from 'teleterm/ui/Document';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import { isDocumentTshNodeWithServerId } from 'teleterm/ui/services/workspacesService';

import Terminal from './Terminal';
import { Reconnect } from './Reconnect';
import { useDocumentTerminal } from './useDocumentTerminal';
import { useTshFileTransferHandlers } from './useTshFileTransferHandlers';

import type * as types from 'teleterm/ui/services/workspacesService';

export function DocumentTerminal(props: {
  doc: types.DocumentTerminal;
  visible: boolean;
}) {
  const ctx = useAppContext();
  const { visible, doc } = props;
  const { attempt, initializePtyProcess } = useDocumentTerminal(doc);
  const { upload, download } = useTshFileTransferHandlers();

  // Initializing a new terminal might fail for multiple reasons, for example:
  //
  // * The user tried to execute `tsh ssh user@host` from the command bar and the request which
  // tries to resolve `host` to a server object failed due to a network or cluster error.
  // * The PTY service has failed to create a new PTY process.
  if (attempt.status === 'error') {
    return (
      <Document visible={props.visible}>
        <Reconnect
          docKind={doc.kind}
          attempt={attempt}
          reconnect={initializePtyProcess}
        />
      </Document>
    );
  }

  const $fileTransfer = doc.kind === 'doc.terminal_tsh_node' && (
    <FileTransferContextProvider>
      <FileTransferActionBar isConnected={doc.status === 'connected'} />
      {isDocumentTshNodeWithServerId(doc) && (
        <FileTransfer
          beforeClose={() =>
            // TODO (gzdunek): replace with a native dialog
            window.confirm('Are you sure you want to cancel file transfers?')
          }
          transferHandlers={{
            getDownloader: async (sourcePath, abortController) => {
              const fileDialog = await ctx.mainProcessClient.showFileSaveDialog(
                sourcePath
              );
              if (fileDialog.canceled) {
                return;
              }
              return download(
                {
                  serverUri: doc.serverUri,
                  login: doc.login,
                  source: sourcePath,
                  destination: fileDialog.filePath,
                },
                abortController
              );
            },
            getUploader: async (destinationPath, file, abortController) =>
              upload(
                {
                  serverUri: doc.serverUri,
                  login: doc.login,
                  source: file.path,
                  destination: destinationPath,
                },
                abortController
              ),
          }}
        />
      )}
    </FileTransferContextProvider>
  );

  return (
    <Document
      visible={visible}
      flexDirection="column"
      pl={2}
      onContextMenu={attempt.data?.openContextMenu}
      autoFocusDisabled={true}
    >
      {$fileTransfer}
      {attempt.status === 'success' && (
        <Terminal
          // The key prop makes sure that we render Terminal only once for each PTY process.
          //
          // When startError occurs and the user initializes a new PTY process, we want to reset all
          // state in <Terminal> and re-run all hooks for the new PTY process.
          key={attempt.data.ptyProcess.getPtyId()}
          docKind={doc.kind}
          ptyProcess={attempt.data.ptyProcess}
          reconnect={initializePtyProcess}
          visible={props.visible}
          onEnterKey={attempt.data.refreshTitle}
        />
      )}
    </Document>
  );
}
