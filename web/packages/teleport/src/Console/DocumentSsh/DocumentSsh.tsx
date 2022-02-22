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
import cfg from 'teleport/config';
import * as Icons from 'design/Icon';
import { Indicator, Text, Box, ButtonPrimary } from 'design';
import * as Alerts from 'design/Alert';
import * as stores from 'teleport/Console/stores';
import FileTransfer, { useFileTransferDialogs } from './../FileTransfer';
import Terminal from './Terminal';
import Document from '../Document';
import AuthnDialog from 'teleport/components/AuthnDialog';
import useWebAuthn from 'teleport/lib/useWebAuthn';
import useSshSession from './useSshSession';
import ActionBar from './ActionBar';

export default function DocumentSsh({ doc, visible }: PropTypes) {
  const refTerminal = useRef<Terminal>();
  const scpDialogs = useFileTransferDialogs();
  const { tty, status, statusText, closeDocument } = useSshSession(doc);
  const webauthn = useWebAuthn(tty);

  function onCloseScpDialogs() {
    scpDialogs.close();
    refTerminal.current.terminal.term.focus();
  }

  useEffect(() => {
    if (refTerminal && refTerminal.current) {
      // when switching tabs or closing tabs, focus on visible terminal
      refTerminal.current.terminal.term.focus();
    }
  }, [visible, webauthn.requested]);

  return (
    <Document visible={visible} flexDirection="column">
      <ActionBar
        isConnected={doc.status === 'connected'}
        isDownloadOpen={scpDialogs.isDownloadOpen}
        isUploadOpen={scpDialogs.isUploadOpen}
        onOpenDownload={scpDialogs.openDownload}
        onOpenUpload={scpDialogs.openUpload}
      />
      {status === 'loading' && (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}
      {status === 'error' && (
        <Alerts.Danger mx="10" mt="5">
          Connection error: {statusText}
        </Alerts.Danger>
      )}
      {status === 'notfound' && (
        <SidNotFoundError sid={doc.sid} clusterId={doc.clusterId} />
      )}
      {webauthn.requested && (
        <AuthnDialog
          onContinue={webauthn.authenticate}
          onCancel={closeDocument}
          errorText={webauthn.errorText}
        />
      )}
      {status === 'initialized' && <Terminal tty={tty} ref={refTerminal} />}
      <FileTransfer
        clusterId={doc.clusterId}
        serverId={doc.serverId}
        login={doc.login}
        isDownloadOpen={scpDialogs.isDownloadOpen}
        isUploadOpen={scpDialogs.isUploadOpen}
        onClose={onCloseScpDialogs}
      />
    </Document>
  );
}

const SidNotFoundError = ({ sid = '', clusterId = '' }) => (
  <Box my={10} mx="auto" width="300px">
    <Text typography="h4" mb="3" textAlign="center">
      The session is no longer active
    </Text>
    <ButtonPrimary
      block
      secondary
      as="a"
      href={cfg.getPlayerRoute({ sid, clusterId }, { recordingType: 'ssh' })}
      target="_blank"
    >
      <Icons.CirclePlay fontSize="5" mr="2" /> Replay Session
    </ButtonPrimary>
  </Box>
);

interface PropTypes {
  doc: stores.DocumentSsh;
  visible: boolean;
}
