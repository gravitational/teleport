var React = require('react');
var ReactSlider = require('react-slider');
var TtyPlayer = require('app/common/ttyPlayer')
var TtyTerminal = require('./../terminal.jsx');
var SessionLeftPanel = require('./sessionLeftPanel');

var SessionPlayer = React.createClass({
  calculateState(){
    return {
      length: this.tty.length,
      min: 1,
      isPlaying: this.tty.isPlaying,
      current: this.tty.current,
      canPlay: this.tty.length > 1
    };
  },

  getInitialState() {
    var sid = this.props.activeSession.sid;
    this.tty = new TtyPlayer({sid});
    return this.calculateState();
  },

  componentWillUnmount() {
    this.tty.stop();
    this.tty.removeAllListeners();
  },

  componentDidMount() {
    this.tty.on('change', ()=>{
      var newState = this.calculateState();
      this.setState(newState);
    });
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
    var {isPlaying} = this.state;

    return (
     <div className="grv-current-session grv-session-player">
       <SessionLeftPanel/>
       <TtyTerminal ref="term" tty={this.tty} cols="5" rows="5" />
       <ReactSlider
          min={this.state.min}
          max={this.state.length}
          value={this.state.current}
          onChange={this.move}
          onAfterChange={this.onAfterChange}
          onBeforeChange={this.onBeforeChange}
          defaultValue={1}
          withBars
          className="grv-slider">
       </ReactSlider>
       <button className="btn" onClick={this.togglePlayStop}>
         { isPlaying ? <i className="fa fa-stop"></i> :  <i className="fa fa-play"></i> }
       </button>
     </div>
     );
  }
});

export default SessionPlayer;
