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
import { withState } from 'shared/hooks';
import { Indicator, Flex, Text, Box, ButtonSecondary } from 'design';
import * as Icons from 'design/Icon';
import cfg from 'teleport/config';
import history from 'teleport/services/history';
import AjaxPoller from 'teleport/components/AjaxPoller';
import FileTransferDialog from './FileTransfer';
import Xterm from './Xterm/Xterm';
import ActionBar from './ActionBar/ActionBar';
import { useStoreSession, useStoreScp } from '../../console';

const POLL_INTERVAL = 3000; // every 3 sec

export class Terminal extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      connected: false,
    };
  }

  componentDidMount() {
    const { sid, storeSession, clusterId } = this.props;
    const { isNew } = storeSession.state;
    if (!isNew) {
      storeSession.joinSession(clusterId, sid);
    }
  }

  replay = () => {
    const { clusterId, sid } = this.props;
    this.props.onOpenPlayer(clusterId, sid);
  };

  onCloseFileTransfer = () => {
    this.props.storeScp.close();
    if (this.termRef) {
      this.termRef.focus();
    }
  };

  onRefresh = () => {
    return this.props.storeSession.fetchParticipants();
  };

  onOpenUploadDialog = () => {
    const params = this.getScpDialogParams();
    this.props.storeScp.openUpload(params);
  };

  onOpenDownloadDialog = () => {
    const params = this.getScpDialogParams();
    this.props.storeScp.openDownload(params);
  };

  onDisconnect = () => {
    this.setState({ connected: false });
  };

  onClose = () => {
    this.props.onClose(this.props.clusterId);
  };

  onSessionStart = () => {
    this.setState({
      connected: true,
    });
  };

  getScpDialogParams() {
    const { clusterId, serverId, login } = this.props.storeSession.state;
    return {
      clusterId,
      serverId,
      login,
    };
  }

  renderXterm(storeSession) {
    const { status } = storeSession.state;
    const title = storeSession.getServerLabel();

    if (status.isLoading) {
      return (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      );
    }

    if (status.isError) {
      return <SidNotFoundError onReplay={this.replay} />;
    }

    if (status.isReady) {
      const termConfig = storeSession.getTtyConfig();
      const { connected } = this.state;
      return (
        <>
          <Xterm
            ref={e => (this.termRef = e)}
            title={title}
            onSessionEnd={this.onDisconnect}
            onSessionStart={this.onSessionStart}
            termConfig={termConfig}
          />
          {connected && (
            <AjaxPoller time={POLL_INTERVAL} onFetch={this.onRefresh} />
          )}
        </>
      );
    }

    return null;
  }

  render() {
    const { storeSession, storeScp } = this.props;
    const title = storeSession.getServerLabel();
    const isFileTransferDialogOpen = storeScp.state.isOpen;
    const $xterm = this.renderXterm(storeSession);

    return (
      <>
        <FileTransferDialog store={storeScp} />
        <Flex flexDirection="column" height="100%" width="100%">
          <Box px={2}>
            <ActionBar
              onOpenUploadDialog={this.onOpenUploadDialog}
              onOpenDownloadDialog={this.onOpenDownloadDialog}
              isFileTransferDialogOpen={isFileTransferDialogOpen}
              title={title}
              onClose={this.onClose}
            />
          </Box>
          <Box px={2} height="100%" width="100%" style={{ overflow: 'auto' }}>
            {$xterm}
          </Box>
        </Flex>
      </>
    );
  }
}

const SidNotFoundError = ({ onReplay }) => (
  <Box my={10} mx="auto" width="300px">
    <Text typography="h4" mb="3" textAlign="center">
      The session is no longer active
    </Text>
    <ButtonSecondary block secondary onClick={onReplay}>
      <Icons.CirclePlay fontSize="5" mr="2" /> Replay Session
    </ButtonSecondary>
  </Box>
);

export default withState(({ match }) => {
  const { clusterId, sid } = match.params;
  const storeSession = useStoreSession();
  const storeScp = useStoreScp();
  return {
    storeSession,
    storeScp,
    onClose: closeWindow,
    onOpenPlayer: openPlayer,
    clusterId,
    sid,
  };
})(Terminal);

function closeWindow() {
  window.close();
}

function openPlayer(clusterId, sid) {
  const routeUrl = cfg.getConsolePlayerRoute({ clusterId, sid });
  history.push(routeUrl);
}
