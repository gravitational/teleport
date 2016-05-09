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

const logger = require('./../logger').create('TtyEvents');

class TtyEvents extends EventEmitter {

  constructor(){
    super();
    this.socket = null;
  }

  connect(connStr){
    this.socket = new WebSocket(connStr);

    this.socket.onopen = () => {
      logger.info('Tty event stream is open');
    }

    this.socket.onmessage = (event) => {
      try
      {
        let json = JSON.parse(event.data);
        this.emit('data', json);
      }
      catch(err){
        logger.error('failed to parse event stream data', err);
      }
    };

    this.socket.onclose = () => {
      logger.info('Tty event stream is closed');
    };
  }

  disconnect(){
    this.socket.close();
  }

}

module.exports = TtyEvents;
