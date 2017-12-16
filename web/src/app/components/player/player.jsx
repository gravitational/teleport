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
import ReactSlider from 'react-slider';
import GrvTerminal from 'app/lib/term/terminal';
import { TtyPlayer } from 'app/lib/term/ttyPlayer';
import { ErrorIndicator } from './items';

initScroll($);

class TerminalPlayer extends GrvTerminal{
  constructor(tty, el){
    super({ el, scrollBack: 1000 });    
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
    // ensure cursor is visible as xterm hides it on blur event
    this.term.cursorState = 1;
    super.resize(cols, rows);        
    $(this._el).perfectScrollbar('update');
  }

  _disconnect(){}

  _requestResize(){}
}

export class Player extends React.Component {

  constructor(props) {
    super(props);
    const { url } = this.props;
    this.tty = new TtyPlayer({url});
    this.state = this.calculateState();
  }

  calculateState(){
    return {
      length: this.tty.length,
      min: 1,
      time: this.tty.getCurrentTime(),
      isPlaying: this.tty.isPlaying,
      isError: this.tty.isError,
      errText: this.tty.errText,
      current: this.tty.current,
      canPlay: this.tty.length > 1
    };
  }
  
  componentDidMount() {
    this.terminal = new TerminalPlayer(this.tty, this.refs.container);
    this.terminal.open();

    this.tty.on('change', this.updateState)
    this.tty.play();
  }
  
  componentWillUnmount() {
    this.tty.stop();
    this.tty.removeAllListeners();
    this.terminal.destroy();
    $(this.refs.container).perfectScrollbar('destroy');
  }

  updateState = () => {
    const newState = this.calculateState();      
    this.setState(newState);
  }

  togglePlayStop = () => {
    if(this.state.isPlaying){
      this.tty.stop();
    }else{
      this.tty.play();
    }
  }

  move = value => {
    this.tty.move(value);
  }
  
  render() {    
    const { isPlaying, isError, errText, time } = this.state;        
    const btnClass = isPlaying ? 'fa fa-stop' : 'fa fa-play';
    if (isError) {
      return <ErrorIndicator text={errText} />
    }

    return (
      <div className="grv-session-player-content">        
        <div ref="container"/>
        <div className="grv-session-player-controls">         
          <button className="btn" onClick={this.togglePlayStop}>
            <i className={btnClass}/>
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
      </div>     
     );
  }
}
