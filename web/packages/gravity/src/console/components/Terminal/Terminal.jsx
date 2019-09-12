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
import { Indicator, Flex, Text, Box, ButtonSecondary } from 'design';
import * as Icons from 'design/Icon';
import { withState } from 'shared/hooks';
import cfg from 'gravity/config';
import history from 'gravity/services/history';
import { getters as termGetters } from 'gravity/console/flux/terminal';
import { getters as fileGetters } from 'gravity/console/flux/scp';
import * as fileActions from 'gravity/console/flux/scp/actions';
import * as termActions from 'gravity/console/flux/terminal/actions';
import { useFluxStore } from 'gravity/components/nuclear';
import AjaxPoller from 'gravity/components/AjaxPoller';
import ActionBar from './ActionBar/ActionBar';
import Xterm from './Xterm/Xterm';
import FileTransferDialog from './FileTransfer';

const POLL_INTERVAL = 3000; // every 5 sec

export class Terminal extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      connected: false,
    };
  }

  componentDidMount() {
    const { sid, onJoin, termStore, siteId } = this.props;
    const { isNew } = termStore;
    if (!isNew) {
      onJoin(siteId, sid);
    }
  }

  replay = () => {
    const { siteId, sid } = this.props;
    this.props.onOpenPlayer(siteId, sid);
  };

  onCloseFileTransfer = () => {
    this.props.onCloseFileTransfer();
    if (this.termRef) {
      this.termRef.focus();
    }
  };

  onRefresh = () => {
    const { siteId, sid } = this.props;
    return this.props.onRefresh({ siteId, sid });
  };

  onOpenUploadDialog = () => {
    const params = this.getScpDialogParams();
    this.props.onOpenUploadDialog(params);
  };

  onOpenDownloadDialog = () => {
    const params = this.getScpDialogParams();
    this.props.onOpenDownloadDialog(params);
  };

  onDisconnect = () => {
    this.setState({ connected: false });
  };

  onClose = () => {
    this.props.onClose(this.props.siteId);
  };

  onSessionStart = () => {
    this.setState({
      connected: true,
    });
  };

  getScpDialogParams() {
    const { siteId, serverId, login } = this.props.termStore;

    return {
      siteId,
      serverId,
      login,
    };
  }

  renderXterm(termStore) {
    const { status } = termStore;
    const title = termStore.getServerLabel();

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
      const termConfig = termStore.getTtyConfig();
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
    const {
      termStore,
      fileStore,
      onTransferUpdate,
      onTransferStart,
      onTransferRemove,
    } = this.props;

    const title = termStore.getServerLabel();
    const isFileTransferDialogOpen = fileStore.isOpen;
    const $xterm = this.renderXterm(termStore);

    return (
      <>
        <FileTransferDialog
          store={fileStore}
          onTransferRemove={onTransferRemove}
          onTransferUpdate={onTransferUpdate}
          onTransferStart={onTransferStart}
          onClose={this.onCloseFileTransfer}
        />
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
  const { siteId, sid } = match.params;
  const termStore = useFluxStore(termGetters.store);
  const fileStore = useFluxStore(fileGetters.store);
  return {
    termStore,
    fileStore,
    onOpenUploadDialog: fileActions.openUploadDialog,
    onOpenDownloadDialog: fileActions.openDownloadDialog,
    onTransferRemove: fileActions.removeFile,
    onTransferStart: fileActions.addFile,
    onTransferUpdate: fileActions.updateStatus,
    onCloseFileTransfer: fileActions.closeDialog,
    onJoin: termActions.joinSession,
    onClose: closeWindow,
    onOpenPlayer: openWindow,
    onRefresh: termActions.fetchSession,
    siteId,
    sid,
  };
})(Terminal);

function closeWindow() {
  window.close();
}

function openWindow(siteId, sid) {
  const routeUrl = cfg.getConsolePlayerRoute({ siteId, sid });
  history.push(routeUrl);
}
