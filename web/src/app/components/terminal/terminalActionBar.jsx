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
import { CloseIcon } from './../icons';
import connect from './../connect';
import { withRouter } from 'react-router';
import { getters as ftGetters } from 'app/flux/fileTransfer';
import * as ftActions from 'app/flux/fileTransfer/actions';
import * as terminalActions from 'app/flux/terminal/actions';
import classnames from 'classnames';

const closeTextStyle = {
  width: "30px",
  height: "30px",
  display: "block",
  margin: "0 auto"
}

class ActionBar extends React.Component {
  openFileTransferDialog = isUpload => {
    const routeParams = this.props.params;
    const params = {
      siteId: routeParams.siteId,
      nodeId: routeParams.serverId,
      login: routeParams.login
    }

    if (isUpload) {
      ftActions.openUploadDialog(params);
    } else {
      ftActions.openDownloadDialog(params);
    }
  }

  close = () => {
    terminalActions.close();
  }

  openUploadDialog = () => {
    this.openFileTransferDialog(true);
  }

  openDownloadDialog = () => {
    this.openFileTransferDialog(false);
  }

  render() {
    const { children, store } = this.props;
    const { isOpen } = store;

    const fileTransferClass = classnames('grv-terminal-actions-files',
      isOpen && '--isOpen'
    );

    return (
      <div className="grv-terminal-actions">
        <div title="Close" style={closeTextStyle} onClick={this.close}>
          <CloseIcon />
        </div>
        <div className="grv-terminal-actions-participans">
          {children}
        </div>
        <div className={fileTransferClass}>
          <a title="Download files"
            className="grv-terminal-actions-files-btn m-b-sm"
            onClick={this.openDownloadDialog}>
            <i className="fa fa-download" />
          </a>
          <a title="Upload files"
            className="grv-terminal-actions-files-btn"
            onClick={this.openUploadDialog}>
            <i className="fa fa-upload" />
          </a>
        </div>
      </div>
    )
  }
}

function mapStateToProps() {
  return {
    store: ftGetters.store,
  }
}

export default connect(mapStateToProps)(withRouter(ActionBar));



