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
import Terminal from 'app/lib/term/terminal';
import { TermEventEnum } from 'app/lib/term/enums';
import termGetters from 'app/flux/terminal/getters';
import TtyAddressResolver from 'app/lib/term/ttyAddressResolver';
import { initTerminal, updateRoute, close } from 'app/flux/terminal/actions';
import * as playerActions from 'app/flux/player/actions';
import PartyListPanel from './../partyListPanel';


import Indicator from './../indicator.jsx';
import PartyList from './terminalPartyList';

class TerminalHost extends React.Component {
    
  constructor(props){
    super(props)        
  }

  componentDidMount() {    
    setTimeout(() => initTerminal(this.props.routeParams), 0);        
  }  

  startNew = () => {
    const newRouteParams = {
      ...this.props.routeParams,
      sid: undefined
    }      
  
    updateRoute(newRouteParams);    
    initTerminal(newRouteParams);    
  }
  
  replay = () => {
    const { siteId, sid } = this.props.routeParams;
    playerActions.open(siteId, sid);
  }

  render() {        
    let { store } = this.props;            
    let { status, sid } = store;
    let serverLabel = store.getServerLabel();
    
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
      document.title = serverLabel;
      $content = (<TerminalContainer store={store}/>)
      $leftPanelContent = (<PartyList sid={sid} />);
    } 
            
    return (
      <div className="grv-terminalhost">
        <PartyListPanel onClose={close}>
          {$leftPanelContent}
        </PartyListPanel>
        <div className="grv-terminalhost-server-info">
           <h3>{serverLabel}</h3>
        </div>
        {$content}               
     </div>
    );
  }
}

class TerminalContainer extends React.Component {
  
  componentDidMount() {                
    const options = this.props.store.getTtyParams();                
    const addressResolver = new TtyAddressResolver(options);    
    this.terminal = new Terminal({
      el: this.refs.container,
      addressResolver      
    });

    this.terminal.open();
    this.terminal.tty.on(TermEventEnum.CLOSE, close);
  }
  
  componentWillUnmount() {
    this.terminal.destroy();
  }

  shouldComponentUpdate() {
    return false;
  }

  render() {
    return ( <div ref="container"/> );
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
    store: termGetters.store      
  }
}

export default connect(mapStateToProps)(TerminalHost);
