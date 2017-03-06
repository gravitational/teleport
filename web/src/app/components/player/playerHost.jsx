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
import $ from 'jQuery';
import initScroll from 'perfect-scrollbar/jquery';
import React from 'react';
import { connect } from 'nuclear-js-react-addons';
import ReactSlider from 'react-slider';
import getters from 'app/modules/player/getters';
import Terminal from 'app/common/term/terminal';
import { TtyPlayer } from 'app/common/term/ttyPlayer';
import { initPlayer, close } from 'app/modules/player/actions';
import Indicator from './../indicator.jsx';
import PartyListPanel from './../partyListPanel';

initScroll($);

const PlayerHost = React.createClass({
    
  componentDidMount() {    
    setTimeout(() => initPlayer(this.props.params), 0);    
  },

  render() {
    let { store } = this.props;    
    if(store.isReady()){
      let url = store.getStoredSessionUrl();
      return <Player url={url}/>;
    }        

    let $indicator = null;

    if(store.isLoading()){
       $indicator = (<Indicator type="bounce" />);
    }        

    if(store.isError()){
       $indicator = (<ErrorIndicator text={store.getErrorText()} />);
    }        
    
    return (
      <Box>{$indicator}</Box>
    );
  }

});

function mapStateToProps() {
  return {    
    store: getters.store
  }
}

export default connect(mapStateToProps)(PlayerHost);

const Player = React.createClass({
  calculateState(){
    return {
      length: this.tty.length,
      min: 1,
      time: this.tty.getCurrentTime(),
      isPlaying: this.tty.isPlaying,
      current: this.tty.current,
      canPlay: this.tty.length > 1
    };
  },

  getInitialState() {
    let { url } = this.props;
    this.tty = new TtyPlayer({url});
    return this.calculateState();
  },

  componentDidMount() {
    this.terminal = new Term(this.tty, this.refs.container);
    this.terminal.open();

    this.tty.on('change', this.updateState)
    this.tty.play();
  },

  updateState(){
    var newState = this.calculateState();
    this.setState(newState);
  },

  componentWillUnmount() {
    this.tty.stop();
    this.tty.removeAllListeners();
    this.terminal.destroy();
    $(this.refs.container).perfectScrollbar('destroy');
  },

  togglePlayStop(){
    if(this.state.isPlaying){
      this.tty.stop();
    }else{
      this.tty.play();
    }
  },

  move(value){
    this.tty.move(value);
  },

  onBeforeChange(){
    this.tty.stop();
  },

  onAfterChange(value){
    this.tty.play();
    this.tty.move(value);
  },

  render: function() {
    var {isPlaying, time} = this.state;

    return (
      <Box>
        <div ref="container"/>
        <div className="grv-session-player-controls">         
         <button className="btn" onClick={this.togglePlayStop}>
           { isPlaying ? <i className="fa fa-stop"></i> :  <i className="fa fa-play"></i> }
         </button>
         <div className="grv-session-player-controls-time">{time}</div>
         <div className="grv-flex-column">
           <ReactSlider
              min={this.state.min}
              max={this.state.length}
              value={this.state.current}
              onChange={this.move}
              defaultValue={1}
              withBars
              className="grv-slider" />
         </div>          
        </div>  
      </Box>     
     );
  }
});

class Term extends Terminal{
  constructor(tty, el){
    super({el, scrollBack: 0});
    this.tty = tty;
  }

  connect(){
    this.tty.connect();
  }

  open() {
    super.open();
    $(this._el).perfectScrollbar();
  }

  resize(cols, rows) {
    if(cols === this.cols && rows === this.rows){
      return;
    }

    super.resize(cols, rows);
    $(this._el).perfectScrollbar('update');
  }

  _disconnect(){}

  _requestResize(){}
}

const Box = props => (
  <div className="grv-terminalhost grv-session-player">
    <PartyListPanel onClose={close} />       
    {props.children}    
  </div>
)

const ErrorIndicator = ({ text }) => (
  <div className="grv-terminalhost-indicator-error">
    <i className="fa fa-exclamation-triangle fa-3x text-warning"></i>
    <div className="m-l">
      <strong>Error</strong>
      <div className="text-center"><small>{text}</small></div>
    </div>
  </div>
)