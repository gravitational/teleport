var React = require('react');
var ReactSlider = require('react-slider');
var TtyPlayer = require('app/common/ttyPlayer')
var TtyTerminal = require('./../terminal.jsx');

var SessionPlayer = React.createClass({
  getStateFromStore(){
    return {
      length: this.tty.length,
      min: 0,
      isPlaying: this.tty.isPlaying,
      current: this.tty.current
    };
  },

  getInitialState() {
    this.tty = new TtyPlayer();
    return this.getStateFromStore();
  },

  componentWillUnmount() {
    this.tty.stop();
    this.tty.removeAllListeners();
  },

  componentDidMount() {
    this.tty.on('change', ()=>{
      var newState = this.getStateFromStore();
      this.setState(newState);
    });
  },

  play(){
    this.tty.play();
  },

  stop(){
    this.tty.stop();
  },

  move(value){
    this.tty.move(value);
  },

  render: function() {
    return (
     <div className="grv-session-player">
       <button className="btn" onClick={this.play}>Play</button>
       <button className="btn" onClick={this.stop}>Stop</button>
       <ReactSlider
          min={this.state.min}
          max={this.state.length}
          value={this.state.current}
          onChange={this.move}
          defaultValue={0}
          withBars
          className="grv-slider">
       </ReactSlider>
       <TtyTerminal ref="term" tty={this.tty} cols="5" rows="5" />
     </div>
     );
  }
});

export default SessionPlayer;
