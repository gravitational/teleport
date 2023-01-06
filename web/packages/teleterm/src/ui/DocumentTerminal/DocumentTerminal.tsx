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

import Terminal from './Terminal';
import DocumentReconnect from './DocumentReconnect';
import useDocTerminal, { Props } from './useDocumentTerminal';
import { useTshFileTransferHandlers } from './useTshFileTransferHandlers';

export default function DocumentTerminalContainer({ doc, visible }: Props) {
  if (doc.kind === 'doc.terminal_tsh_node' && doc.status === 'disconnected') {
    return <DocumentReconnect visible={visible} doc={doc} />;
  }

  return <DocumentTerminal visible={visible} doc={doc} />;
}

export function DocumentTerminal(props: Props & { visible: boolean }) {
  const ctx = useAppContext();
  const { visible, doc } = props;
  const state = useDocTerminal(doc);
  const ptyProcess = state.data?.ptyProcess;
  const { upload, download } = useTshFileTransferHandlers();

  return (
    <Document
      visible={visible}
      flexDirection="column"
      pl={2}
      onContextMenu={state.data?.openContextMenu}
      autoFocusDisabled={true}
    >
      {ptyProcess && (
        <>
          {doc.kind === 'doc.terminal_tsh_node' && (
            <FileTransferContextProvider>
              <FileTransferActionBar isConnected={doc.status === 'connected'} />
              <FileTransfer
                beforeClose={() =>
                  // TODO (gzdunek): replace with a native dialog
                  window.confirm(
                    'Are you sure you want to cancel file transfers?'
                  )
                }
                transferHandlers={{
                  getDownloader: async (sourcePath, abortController) => {
                    const fileDialog =
                      await ctx.mainProcessClient.showFileSaveDialog(
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
            </FileTransferContextProvider>
          )}
          <Terminal
            ptyProcess={ptyProcess}
            visible={props.visible}
            onEnterKey={state.data.refreshTitle}
          />
        </>
      )}
    </Document>
  );
}
