var session = require('app/session');
var cfg = require('app/config');
var React = require('react');
var {getters, actions} = require('app/modules/activeTerminal/');
var EventStreamer = require('./eventStreamer.jsx');

var ActiveSession = React.createClass({

  mixins: [reactor.ReactMixin],

  componentDidMount: function() {
    //actions.open
    //actions.open(data[rowIndex].addr, user.logins[i], undefined)}>{user.logins[i]}</a></li>);
  },

  onOpen(){
    actions.connected();
  },

  getDataBindings() {
    return {
      activeSession: getters.activeSession
    }
  },

  render: function() {
    if(!this.state.activeSession){
      return null;
    }

    var {isConnected, ...settings} = this.state.activeSession;
    var {token} = session.getUserData();

    return (
     <div className="grv-terminal-host">
       <div className="grv-terminal-participans">
         <ul className="nav">
           <li><button className="btn btn-primary btn-circle" type="button"> <strong>A</strong></button></li>
           <li><button className="btn btn-primary btn-circle" type="button"> B </button></li>
           <li><button className="btn btn-primary btn-circle" type="button"> C </button></li>
           <li>
             <button onClick={actions.close} className="btn btn-danger btn-circle" type="button">
               <i className="fa fa-times"></i>
             </button>
           </li>
         </ul>
       </div>
       <div>
         <div className="btn-group">
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
         </div>
       </div>
       { isConnected ? <EventStreamer token={token} sid={settings.sid}/> : null }
       <TerminalBox settings={settings} token={token} onOpen={actions.connected}/>
     </div>
     );
  }
});

var TerminalBox = React.createClass({
  renderTerminal: function() {
    var parent = document.getElementById("terminal-box");
    var {settings, token, sid } = this.props;

    //settings.sid = 5555;
    settings.term = {
      h: 120,
      w: 100
    };

    var connectionStr = cfg.api.getSessionConnStr(token, settings);

    this.term = new Terminal({
      cols: 180,
      rows: 50,
      useStyle: true,
      screenKeys: true,
      cursorBlink: false
    });

    this.term.open(parent);
    this.socket = new WebSocket(connectionStr, "proto");
    this.term.write('\x1b[94mconnecting to "pod"\x1b[m\r\n');

    this.socket.onopen = () => {
      this.props.onOpen();
      this.term.on('data', (data) => {
        this.socket.send(data);
      });

      this.socket.onmessage = (e) => {
        this.term.write(e.data);
      }

      this.socket.onclose = () => {
        this.term.write('\x1b[31mdisconnected\x1b[m\r\n');
      }
    }
  },

  componentDidMount: function() {
    this.renderTerminal();
  },

  componentWillUnmount: function() {
    this.socket.close();
    this.term.destroy();
  },

  shouldComponentUpdate: function() {
    return false;
  },

  componentWillReceiveProps: function(props) {
  },

  render: function() {
    return (
        <div className="grv-wiz-terminal" id="terminal-box">
        </div>
    );
  }
});

export default ActiveSession;
export {TerminalBox, ActiveSession};
