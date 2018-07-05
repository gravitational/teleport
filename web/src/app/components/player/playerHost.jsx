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
import { close } from 'app/flux/player/actions';
import { Player } from './player';
import { DocumentTitle } from './../documentTitle';
import { CloseIcon } from './../icons';
import cfg from 'app/config';

class PlayerHost extends React.Component {

  componentWillMount() {
    const { sid, siteId } = this.props.params;
    this.url = cfg.api.getFetchSessionUrl({ siteId, sid });
  }

  render() {
    if (!this.url) {
      return null;
    }

    const { siteId } = this.props.params;
    const title = `${siteId} · Player`;
    return (
      <DocumentTitle title={title}>
        <div className="grv-terminalhost grv-session-player">
          <div className="grv-session-player-actions m-t-md">
            <div title="Close" style={closeTextStyle} onClick={close}>
              <CloseIcon />
            </div>
          </div>
          <Player url={this.url}/>
        </div>
      </DocumentTitle>
    );
  }
}

const closeTextStyle = {
  width: "30px",
  height: "30px",
  display: "block",
  margin: "0 auto"
}

export default PlayerHost;