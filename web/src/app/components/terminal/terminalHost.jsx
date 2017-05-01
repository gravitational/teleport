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
import PartyListPanel from './../partyListPanel';
import Terminal from 'app/lib/term/terminal';
import termGetters from 'app/flux/terminal/getters';
import Indicator from './../indicator.jsx';
import { initTerminal, updateRoute, close } from 'app/flux/terminal/actions';
import { updateSession } from 'app/flux/sessions/actions';
import { openPlayer } from 'app/flux/player/actions';
import PartyList from './terminalPartyList';
import { EventTypeEnum } from 'app/lib/term/enums';
import { connect } from 'nuclear-js-react-addons';

class TerminalHost extends React.Component {
    
  constructor(props){
    super(props)        
  }

  componentDidMount() {    
    setTimeout(() => initTerminal(this.props.routeParams), 0);        
  }  

  startNew = () => {
    let newRouteParams = {
      ...this.props.routeParams,
      sid: undefined
    }      
  
    updateRoute(newRouteParams);    
    initTerminal(newRouteParams);    
  }

  replay = () => {
    openPlayer(this.props.routeParams);
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
    let options = {
      tty: this.props.store.getTtyParams(),  
      el: this.refs.container
    }    
    this.terminal = new Terminal(options);
    this.terminal.ttyEvents.on('data', this.receiveEvents.bind(this));
    this.terminal.open();
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

  receiveEvents(data) {        
    // loop through events in reverse order to find the last resize event to sync the screen size.    
    data.events.reverse().every(item => {
      let { event, size } = item;

      // exit terminal if session is ended
      if (event === EventTypeEnum.END) {
        close();
      }

      if (event === EventTypeEnum.RESIZE) {
        let [w, h] = size.split(':');        
        this.terminal.resize(Number(w), Number(h));
        return false;
      }

      return true;
    });

    updateSession({      
      siteId: this.props.siteId,
      json: data.session      
    })                                  
  }
}

const ErrorIndicator = ({ text }) => (
  <div className="grv-terminalhost-indicator-error">
    <i className="fa fa-exclamation-triangle fa-3x text-warning"></i>
    <div className="m-l">
      <strong>Connection error</strong>
      <div className="text-center"><small>{text}</small></div>
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
