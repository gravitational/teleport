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
import ReactDOM from 'react-dom';
import ReactSlider from 'react-slider';
import GrvTerminal from 'app/lib/term/terminal';
import { TtyPlayer } from 'app/lib/term/ttyPlayer';
import Indicator from './../indicator.jsx';
import { ErrorIndicator, WarningIndicator } from './items';

initScroll($);

class Terminal extends GrvTerminal{
  constructor(tty, el){
    super({ el, scrollBack: 1000 });    
    this.tty = tty;            
  }

  connect(){    
  }

  open() {
    super.open();              
    $(this._el).perfectScrollbar();
  }

  resize(cols, rows) {           
    // ensure that cursor is visible as xterm hides it on blur event
    this.term.cursorState = 1;
    super.resize(cols, rows);        
    $(this._el).perfectScrollbar('update');
  }

  destroy() {
    super.destroy();
    $(this._el).perfectScrollbar('destroy');
  }

  _disconnect(){}

  _requestResize(){}
}

class Content extends React.Component {

  static propTypes = {
    tty: React.PropTypes.object.isRequired    
  }

  componentDidMount() {    
    const tty = this.props.tty;
    this.terminal = new Terminal(tty, this.refs.container);
    this.terminal.open();    
  }
  
  componentWillUnmount() {    
    this.terminal.destroy();        
  }

  render() {
    const isLoading = this.props.tty.isLoading;
    // need to hide the terminal cursor while fetching for events
    const style = {
      visibility: isLoading ? "hidden" : "initial"
    }

    return (<div style={style} ref="container" />);
  }
}

class ControlPanel extends React.Component {
    
  componentDidMount() {    
    const el = ReactDOM.findDOMNode(this)
    const btn = el.querySelector('.grv-session-player-controls button');
    btn && btn.focus();    
  }
     
  render() {
    const { isPlaying, min, max, value, onChange, onToggle, time } = this.props;      
    const btnClass = isPlaying ? 'fa fa-stop' : 'fa fa-play';
    return (
      <div className="grv-session-player-controls">
        <button className="btn" onClick={onToggle}>
          <i className={btnClass} />
        </button>
        <div className="grv-session-player-controls-time">{time}</div>
        <div className="grv-flex-column">
          <ReactSlider
            min={min}
            max={max}
            value={value}
            onChange={onChange}
            defaultValue={1}
            withBars
            className="grv-slider" />
        </div>
      </div>
    )  
  }
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
      eventCount: this.tty.getEventCount(),
      length: this.tty.length,
      min: 1,
      time: this.tty.getCurrentTime(),      
      isLoading: this.tty.isLoading,
      isPlaying: this.tty.isPlaying,
      isError: this.tty.isError,
      errText: this.tty.errText,
      current: this.tty.current,
      canPlay: this.tty.length > 1
    };
  }
  
  componentDidMount() {        
    this.tty.on('change', this.updateState)    
    this.tty.connect();
    this.tty.play();
  }
  
  componentWillUnmount() {
    this.tty.stop();
    this.tty.removeAllListeners();    
  }

  updateState = () => {
    const newState = this.calculateState();      
    this.setState(newState);
  }

  onTogglePlayStop = () => {
    if(this.state.isPlaying){
      this.tty.stop();
    }else{
      this.tty.play();
    }
  }

  onMove = value => {
    this.tty.move(value);
  }
  
  render() {    
    const {
      isPlaying,
      isLoading,
      isError,
      errText,
      time,
      min,
      length,
      current,      
      eventCount
    } = this.state;
        
    if (isError) {
      return <ErrorIndicator text={errText} />
    }
    
    if (!isLoading && eventCount === 0 ) {
      return <WarningIndicator text="The recording for this session is not available." />
    }
        
    return (
      <div className="grv-session-player-content">                
        <Content tty={this.tty} />    
        {isLoading && <Indicator />}            
        {eventCount > 0 && (
          <ControlPanel 
            isPlaying={isPlaying}
            time={time}
            min={min}            
            max={length}
            value={current}
            onToggle={this.onTogglePlayStop}
            onChange={this.onMove}/>)                              
        }  
      </div>     
     );
  }
}