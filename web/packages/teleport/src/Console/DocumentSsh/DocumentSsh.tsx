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

import React, { useRef, useEffect } from 'react';

import { Indicator, Box } from 'design';

import {
  FileTransferActionBar,
  FileTransfer,
  FileTransferContextProvider,
  FileTransferContainer,
  FileTransferRequests,
} from 'shared/components/FileTransfer';

import * as stores from 'teleport/Console/stores';
import { colors } from 'teleport/Console/colors';

import AuthnDialog from 'teleport/components/AuthnDialog';

import Document from '../Document';

import Terminal from './Terminal';
import useSshSession from './useSshSession';

export default function DocumentSsh({ doc, visible }: PropTypes) {
  const refTerminal = useRef<Terminal>();
  const {
    tty,
    status,
    closeDocument,
    session,
    fileTransferRequestResponse,
    webauthn,
    upload,
    download,
    getMfaResponseAttempt,
    filesStore,
    fileTransferRequests,
  } = useSshSession(doc);

  function handleCloseFileTransfer() {
    refTerminal.current.terminal.term.focus();
  }

  useEffect(() => {
    if (refTerminal?.current) {
      // when switching tabs or closing tabs, focus on visible terminal
      refTerminal.current.terminal.term.focus();
    }
  }, [visible, webauthn.requested]);

  return (
    <Document visible={visible} flexDirection="column">
      <FileTransferContextProvider>
        <FileTransferActionBar isConnected={doc.status === 'connected'} />
        {status === 'loading' && (
          <Box textAlign="center" m={10}>
            <Indicator />
          </Box>
        )}
        {webauthn.requested && (
          <AuthnDialog
            onContinue={webauthn.authenticate}
            onCancel={closeDocument}
            errorText={webauthn.errorText}
          />
        )}
        {status === 'initialized' && <Terminal tty={tty} ref={refTerminal} />}
        <FileTransferContainer>
          {fileTransferRequests.length > 0 && (
            <FileTransferRequests
              onDeny={fileTransferRequestResponse}
              onApprove={fileTransferRequestResponse}
              requests={fileTransferRequests}
            />
          )}
          <FileTransfer
            filesStore={filesStore}
            beforeClose={() =>
              window.confirm('Are you sure you want to cancel file transfers?')
            }
            errorText={
              getMfaResponseAttempt.status === 'failed'
                ? getMfaResponseAttempt.statusText
                : null
            }
            afterClose={handleCloseFileTransfer}
            backgroundColor={colors.levels.surface}
            transferHandlers={{
              getDownloader: async (location, abortController) => {
                if (session.isModeratedSession) {
                  tty.sendFileTransferRequest(location, 'download');
                  return;
                }
                return download(location, abortController);
              },
              getUploader: async (location, file, abortController) => {
                return upload(location, file, abortController);
              },
            }}
          />
        </FileTransferContainer>
      </FileTransferContextProvider>
    </Document>
  );
}

interface PropTypes {
  doc: stores.DocumentSsh;
  visible: boolean;
}
