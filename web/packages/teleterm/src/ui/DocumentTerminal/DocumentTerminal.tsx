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
import { Flex, Text, ButtonPrimary } from 'design';
import { Danger } from 'design/Alert';
import {
  FileTransferActionBar,
  FileTransfer,
  FileTransferContextProvider,
} from 'shared/components/FileTransfer';
import { Attempt } from 'shared/hooks/useAsync';

import Document from 'teleterm/ui/Document';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import { assertUnreachable } from 'teleterm/ui/utils';
import { isDocumentTshNodeWithServerId } from 'teleterm/ui/services/workspacesService';

import { Terminal } from './Terminal';
import { Props, useDocumentTerminal } from './useDocumentTerminal';
import { useTshFileTransferHandlers } from './useTshFileTransferHandlers';

import type * as types from 'teleterm/ui/services/workspacesService';

export function DocumentTerminal(props: Props & { visible: boolean }) {
  const ctx = useAppContext();
  const { configService } = ctx.mainProcessClient;
  const { visible, doc } = props;
  const { attempt, reconnect } = useDocumentTerminal(doc);
  const ptyProcess = attempt.data?.ptyProcess;
  const { upload, download } = useTshFileTransferHandlers();
  const unsanitizedTerminalFontFamily = configService.get(
    'terminal.fontFamily'
  ).value;
  const terminalFontSize = configService.get('terminal.fontSize').value;

  // Creating a new terminal might fail for multiple reasons, for example:
  //
  // * The user tried to execute `tsh ssh user@host` from the command bar and the request which
  // tries to resolve `host` to a server object failed due to a network or cluster error.
  // * The PTY service has failed to create a new PTY process.
  if (attempt.status === 'error') {
    return (
      <DocumentReconnect
        visible={visible}
        doc={doc}
        attempt={attempt}
        reconnect={reconnect}
      />
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
      {ptyProcess && (
        <Terminal
          ptyProcess={ptyProcess}
          visible={props.visible}
          unsanitizedFontFamily={unsanitizedTerminalFontFamily}
          fontSize={terminalFontSize}
          onEnterKey={attempt.data.refreshTitle}
        />
      )}
    </Document>
  );
}

function DocumentReconnect(props: {
  visible: boolean;
  doc: types.DocumentTerminal;
  attempt: Attempt<unknown>;
  reconnect: () => void;
}) {
  const { message, buttonText } = getReconnectCopy(props.doc);

  return (
    <Document visible={props.visible} flexDirection="column" pl={2}>
      <Flex
        gap={4}
        flexDirection="column"
        mx="auto"
        alignItems="center"
        mt={100}
      >
        <Text typography="h5" color="text.primary">
          {message}
        </Text>
        <Flex flexDirection="column" alignItems="center" mx="auto">
          <Danger mb={3}>{props.attempt.statusText}</Danger>
          <ButtonPrimary width="100px" onClick={props.reconnect}>
            {buttonText}
          </ButtonPrimary>
        </Flex>
      </Flex>
    </Document>
  );
}

function getReconnectCopy(doc: types.DocumentTerminal) {
  switch (doc.kind) {
    case 'doc.terminal_tsh_node': {
      return {
        message: 'This SSH connection is currently offline.',
        buttonText: 'Reconnect',
      };
    }
    case 'doc.terminal_shell':
    case 'doc.terminal_tsh_kube': {
      return {
        message: 'Ran into an error when starting the terminal session.',
        buttonText: 'Retry',
      };
    }
    default:
      assertUnreachable(doc);
  }
}
