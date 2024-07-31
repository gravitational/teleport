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
import {
  FileTransferActionBar,
  FileTransfer,
  FileTransferContextProvider,
} from 'shared/components/FileTransfer';

import Document from 'teleterm/ui/Document';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import { isDocumentTshNodeWithServerId } from 'teleterm/ui/services/workspacesService';

import { Terminal } from './Terminal';
import { Reconnect } from './Reconnect';
import { useDocumentTerminal } from './useDocumentTerminal';
import { useTshFileTransferHandlers } from './useTshFileTransferHandlers';

import type * as types from 'teleterm/ui/services/workspacesService';

export function DocumentTerminal(props: {
  doc: types.DocumentTerminal;
  visible: boolean;
}) {
  const ctx = useAppContext();
  const { configService } = ctx.mainProcessClient;
  const { visible, doc } = props;
  const { attempt, initializePtyProcess } = useDocumentTerminal(doc);
  const { upload, download } = useTshFileTransferHandlers();
  const unsanitizedTerminalFontFamily = configService.get(
    'terminal.fontFamily'
  ).value;
  const terminalFontSize = configService.get('terminal.fontSize').value;

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
              const fileDialog =
                await ctx.mainProcessClient.showFileSaveDialog(sourcePath);
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
      // adds some space from the top so the shell content is not covered by a shadow
      pt={1}
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
          unsanitizedFontFamily={unsanitizedTerminalFontFamily}
          fontSize={terminalFontSize}
          onEnterKey={attempt.data.refreshTitle}
          windowsPty={attempt.data.windowsPty}
          keyboardShortcutsService={ctx.keyboardShortcutsService}
        />
      )}
    </Document>
  );
}
