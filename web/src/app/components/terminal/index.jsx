/*
Copyright 2015 Gravitational, Inc.

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
import { connect } from 'nuclear-js-react-addons';
import termGetters from 'app/flux/terminal/getters';
import { getters as fileGetters } from 'app/flux/fileTransfer';
import * as terminalActions  from 'app/flux/terminal/actions';
import * as playerActions from 'app/flux/player/actions';
import * as fileActions from 'app/flux/fileTransfer/actions';
import LeftMenu from './terminalActionBar';
import Indicator from './../indicator.jsx';
import PartyList from './terminalPartyList';
import { Terminal } from './terminal';
import { FileTransferDialog } from './../files';

class Page extends React.Component {

  constructor(props){
    super(props)
  }

  componentDidMount() {
    setTimeout(() => terminalActions.initTerminal(this.props.routeParams), 0);
  }

  startNew = () => {
    const newRouteParams = {
      ...this.props.routeParams,
      sid: undefined
    }

    terminalActions.updateRoute(newRouteParams);
    terminalActions.initTerminal(newRouteParams);
  }

  replay = () => {
    const { siteId, sid } = this.props.routeParams;
    playerActions.open(siteId, sid);
  }

  onCloseFileTransfer = () => {
    fileActions.closeDialog();
    if (this.termRef) {
      this.termRef.focus();
    }
  }

  render() {
    const { termStore, fileStore } = this.props;
    const { status, sid } = termStore;
    const title = termStore.getServerLabel();

    let $content = null;
    let $leftPanelContent = null;

    if (status.isLoading) {
      $content = (<Indicator type="bounce" />);
    }

    if (status.isError) {
      $content = (<ErrorIndicator text={status.errorText} />);
    }

    if (status.isNotFound) {
      $content = (
        <SidNotFoundError
          onReplay={this.replay}
          onNew={this.startNew} />);
    }

    if (status.isReady) {
      const ttyParams = termStore.getTtyParams();
      $content = (
        <Terminal ref={e => this.termRef = e}
          title={title}
          onSessionEnd={terminalActions.close}
          ttyParams={ttyParams} />
      );
      $leftPanelContent = (<PartyList sid={sid} />);
    }

    return (
      <div>
        <FileTransferDialog
          store={fileStore}
          onClose={this.onCloseFileTransfer}
          onTransfer={fileActions.addFile}
        />
        <div className="grv-terminalhost">
          <LeftMenu>
            {$leftPanelContent}
          </LeftMenu>
          <div className="grv-terminalhost-server-info">
            <h3>{title}</h3>
          </div>
          {$content}
        </div>
      </div>
    );
  }
}

const ErrorIndicator = ({ text }) => (
  <div className="grv-terminalhost-indicator-error">
    <i className="fa fa-exclamation-triangle fa-3x text-warning"></i>
    <div className="m-l">
      <strong>Connection error</strong>
      <div><small>{text}</small></div>
    </div>
  </div>
)

const SidNotFoundError = ({onNew, onReplay}) => (
  <div className="grv-terminalhost-indicator-error">
    <div className="text-center">
      <strong>The session is no longer active</strong>
      <div className="m-t">
        <button onClick={onNew} className="btn btn-sm btn-primary m-r"> Start New </button>
        <button onClick={onReplay} className="btn btn-sm btn-primary"> Replay </button>
      </div>
    </div>
  </div>
)

function mapStateToProps() {
  return {
    termStore: termGetters.store,
    fileStore: fileGetters.store
  }
}

export default connect(mapStateToProps)(Page);
