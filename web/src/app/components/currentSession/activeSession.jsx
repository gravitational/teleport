var React = require('react');
var {getters, actions} = require('app/modules/activeTerminal/');
var Tty = require('app/common/tty');
var TtyTerminal = require('./../terminal.jsx');
var EventStreamer = require('./eventStreamer.jsx');
var SessionLeftPanel = require('./sessionLeftPanel');

var ActiveSession = React.createClass({
  render: function() {
    return (
     <div className="grv-current-session">
       <SessionLeftPanel/>
       <div>
         {/*<div className="btn-group">
           <span className="btn btn-xs btn-primary">128.0.0.1:8888</span>
           <div className="btn-group">
             <button data-toggle="dropdown" className="btn btn-default btn-xs dropdown-toggle" aria-expanded="true">
               <span className="caret"></span>
             </button>
             <ul className="dropdown-menu">
               <li><a href="#" target="_blank">Logs</a></li>
               <li><a href="#" target="_blank">Logs</a></li>
             </ul>
           </div>
         </div>*/}
       </div>
       <TtyConnection {...this.props.activeSession} />
     </div>
     );
  }
});

var TtyConnection = React.createClass({

  getInitialState() {
    this.tty = new Tty(this.props)
    this.tty.on('open', ()=> this.setState({ isConnected: true }));
    return {isConnected: false};
  },

  componentWillUnmount() {
    this.tty.disconnect();
  },

  render() {

    return (
      <div style={{height: '100%'}}>
        <TtyTerminal tty={this.tty} cols={this.props.cols} rows={this.props.rows} />
        { this.state.isConnected ? <EventStreamer sid={this.props.sid}/> : null }
      </div>
    )
  }
});

module.exports = ActiveSession;
