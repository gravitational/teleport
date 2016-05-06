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

var EventEmitter = require('events').EventEmitter;

class Tty extends EventEmitter {

  constructor(){
    super();
    this.socket = null;
  }

  disconnect(){
    this.socket.close();
  }

  reconnect(options){
    this.disconnect();
    this.socket.onopen = null;
    this.socket.onmessage = null;
    this.socket.onclose = null;

    this.connect(options);
  }

  connect(connStr){
    this.socket = new WebSocket(connStr);

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

  send(data){
    this.socket.send(data);
  }
}

module.exports = Tty;
