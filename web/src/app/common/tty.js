var EventEmitter = require('events').EventEmitter;
var session = require('app/session');
var cfg = require('app/config');
var {actions} = require('app/modules/activeTerminal/');

class Tty extends EventEmitter {

  constructor({addr, login, sid, rows, cols }){
    super();
    this.options = { addr, login, sid, rows, cols };
    this.socket = null;
  }

  disconnect(){
    this.socket.close();
  }

  connect(options){
    Object.assign(this.options, options);

    let {token} = session.getUserData();
    let connStr = cfg.api.getTtyConnStr({token, ...this.options});

    this.socket = new WebSocket(connStr, 'proto');

    this.socket.onopen = () => {
      this.emit('open');
    }

    this.socket.onmessage = (e)=>{
      this.emit('data', e.data);
    }

    this.socket.onclose = ()=>{
      this.emit('close');
    }
  }

  resize(cols, rows){
    actions.resize(cols, rows);
  }

  send(data){
    this.socket.send(data);
  }
}

module.exports = Tty;
