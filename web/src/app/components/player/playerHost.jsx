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
import getters from 'app/flux/player/getters';
import { initPlayer, close } from 'app/flux/player/actions';
import Indicator from './../indicator.jsx';
import { ErrorIndicator } from './items';
import { Player } from './player';
import PartyListPanel from './../partyListPanel';

class PlayerHost extends React.Component {
    
  componentDidMount() {    
    setTimeout(() => initPlayer(this.props.params), 0);    
  }

  render() {
    const { store } = this.props;    
    const isReady = store.isReady();
    const isLoading = store.isLoading();
    const isError = store.isError();
    const errText = store.getErrorText();
    const url = store.getStoredSessionUrl();
    
    return (
      <div className="grv-terminalhost grv-session-player">
        <PartyListPanel onClose={close} />         
        {isLoading && <Indicator type="bounce" />}
        {isError && <ErrorIndicator text={errText} />}
        {isReady &&  <Player url={url}/>}
      </div>
    );
  }  
}

function mapStateToProps() {
  return {    
    store: getters.store
  }
}

export default connect(mapStateToProps)(PlayerHost);