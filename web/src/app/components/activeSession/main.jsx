var session = require('app/session');
var cfg = require('app/config');
var React = require('react');
var {getters, actions} = require('app/modules/activeTerminal/');
var EventStreamer = require('./eventStreamer.jsx');
var {debounce} = require('_');

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
       { isConnected ? <EventStreamer sid={settings.sid}/> : null }
       <TerminalBox settings={settings} onOpen={actions.connected}/>
     </div>
     );
  }
});

Terminal.colors[256] = 'inherit';

var TerminalBox = React.createClass({
  resize: function(e) {
    this.destroy();
    this.renderTerminal();
  },

  connect(cols, rows){
    let {token} = session.getUserData();
    let {settings, sid } = this.props;

    settings.term = {
      h: rows,
      w: cols
    };

    let connectionStr = cfg.api.getSessionConnStr(token, settings);

    this.socket = new WebSocket(connectionStr, 'proto');
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

  getDimensions(){
    var $container = $(this.refs.container);
    var $term = $container.find('.terminal');
    var cols, rows;

    if($term.length === 1){
      var fakeRow = $('<div><span>&nbsp;</span></div>');
      $term.append(fakeRow);
      var fakeCol = fakeRow.children().first()[0].getBoundingClientRect();
      cols = Math.floor($term.width() / (fakeCol.width || 9));
      rows = Math.floor($term.height() / (fakeCol.height || 20));
      fakeRow.remove();
    }else{
      var $container = $(this.refs.container);
      cols = Math.floor($container.width() / 9);
      rows = Math.floor($container.height() / 20);
    }

    return {cols, rows};

  },

  destroy(){
    if(this.socket){
      this.socket.close();
    }

    this.term.destroy();
  },

  renderTerminal() {
    var {cols, rows} = this.getDimensions();
    var {settings, token, sid } = this.props;

    this.term = new Terminal({
      cols,
      rows,
      useStyle: true,
      screenKeys: true,
      cursorBlink: true
    });

    this.term.open(this.refs.container);
    this.term.write('\x1b[94mconnecting...\x1b[m\r\n');
    this.connect(cols, rows);
  },

  componentDidMount: function() {
    this.renderTerminal();
    this.resize = debounce(this.resize, 100);
    window.addEventListener('resize', this.resize);
  },

  componentWillUnmount: function() {
    this.destroy();
    window.removeEventListener('resize', this.resize);
  },

  shouldComponentUpdate: function() {
    return false;
  },

  render: function() {
    return (
        <div className="grv-terminal" id="terminal-box" ref="container">
        </div>
    );
  }
});

export default ActiveSession;
export {TerminalBox, ActiveSession};
