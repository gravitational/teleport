var session = require('app/session');
var cfg = require('app/config');
var React = require('react');
var {getters, actions} = require('app/modules/activeTerminal/');

var TerminalHost = React.createClass({

  mixins: [reactor.ReactMixin],

  getDataBindings() {
    return {
      terminal: getters.terminal
    }
  },

  render: function() {
    if(!this.state.terminal){
      return null;
    }

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
       <TerminalBox settings={this.state.terminal} />
     </div>
     );
  }
});

var TerminalBox = React.createClass({
  renderTerminal: function() {
    var {token} = session.getUserData();
    var parent = document.getElementById("terminal-box");

    var settings = this.props.settings;
    //settings.sid = 5555;
    settings.term = {
      h: 120,
      w: 100
    };

    var connectionStr = cfg.api.getTermConnString(token, settings);

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

export default TerminalHost;
export {TerminalBox, TerminalHost};
