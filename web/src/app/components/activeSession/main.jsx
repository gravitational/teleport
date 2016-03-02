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
    var {cols, rows } = getDimensions(this.refs.container);
    this.term.resize(cols, rows);
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

    this.socket.onmessage = (e) => {
      this.term.write(e.data);
    };

    this.socket.onclose = () => {
      this.term.write('\x1b[31mdisconnected\x1b[m\r\n');
    };

    this.socket.onopen = () => {
      this.props.onOpen();
      this.term.on('data', (data) => {
        this.socket.send(data);
      });
    }
  },

  destroy(){
    if(this.socket){
      this.socket.close();
    }

    this.term.destroy();
  },

  renderTerminal() {
    var {cols, rows} = getDimensions(this.refs.container);
    var {settings, token, sid } = this.props;

    this.term = new Terminal({
      cols,
      rows,
      useStyle: true,
      screenKeys: true,
      cursorBlink: true
    });

    this.term.open(this.refs.container);
    this.connect(cols, rows);
    this.resize();
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

function getDimensions(container){
  var $container = $(container);
  var $term = $container.find('.terminal');
  var cols, rows;

  if($term.length === 1){
    let fakeRow = $('<div><span>&nbsp;</span></div>');
    $term.append(fakeRow);
    // get div height
    let fakeColHeight = fakeRow[0].getBoundingClientRect().height;
    // get span width
    let fakeColWidth = fakeRow.children().first()[0].getBoundingClientRect().width;
    cols = Math.floor($container.width() / (fakeColWidth || 9));
    rows = Math.floor($container.height() / (fakeColHeight|| 20));
    fakeRow.remove();
  }else{
    // some default values (just to init)
    cols = Math.floor($container.width() / 9) - 1;
    rows = Math.floor($container.height() / 20);
  }

  return {cols, rows};
}

export default ActiveSession;
export {TerminalBox, ActiveSession};


/*

/*var TtyConnection = React.createClass({

  getInitialState: function() {
    return {
      isConnected: false,
      isConnecting: true,
      msg: 'Connecting...'
    }
  },

  componentDidMount: function() {
    let {token} = session.getUserData();
    let {settings, sid } = this.props;

    settings.term = {
      h: rows,
      w: cols
    };

    let connectionStr = cfg.api.getSessionConnStr(token, settings);
    this.socket = new WebSocket(connectionStr, 'proto');

    this.socket.onmessage = (e)=>{
      this.setState({
        ...this.state,
        data: e.data
      })
    }

    this.socket.onclose = ()=>{
      this.setState({
        ...this.state,
        isConnected: false,
        data: 'disconneted'
      })
    };

    this.socket.onopen = () => {
      this.setState({
        isConnected: true
      });
    }
  },

  componentWillUnmount() {
    if(this.socket){
      this.socket.close();
    }
  },

  shouldComponentUpdate() {
    return false;
  },

  render() {
    return {...this.props.children};
  }
});
*/
