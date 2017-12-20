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

import { EventEmitter } from 'events';
import { sortBy } from 'lodash';
import { StatusCodeEnum, EventTypeEnum } from './enums';
import Logger from './../logger';

const logger = Logger.create('TtyEvents');

class TtyEvents extends EventEmitter {

  socket = null;

  _addressResolver = null;

  constructor(addressResolver){
    super();        
    this._addressResolver = addressResolver;
  }

  connect(){
    const connStr = this._addressResolver.getEventProviderConnStr();
    this.socket = new WebSocket(connStr);
    this.socket.onmessage = this._onReceiveMessage.bind(this);
    this.socket.onclose = this._onCloseConnection.bind(this);
    this.socket.onopen = () => {
      logger.info('websocket is open');
    }        
  }

  disconnect(reasonCode = StatusCodeEnum.NORMAL) {
    if (this.socket !== null) {
      this.socket.close(reasonCode);
    }  
  }

 _onCloseConnection(e) {
    this.socket.onmessage = null;
    this.socket.onopen = null;
    this.socket.onclose = null;
    this.emit('close', e);      
    logger.info('websocket is closed');
  }

  _onReceiveMessage(message) {
    try
    {
      let json = JSON.parse(message.data);          
      this._processResize(json.events)
      this.emit('data', json);
    }
    catch(err){
      logger.error('failed to parse event stream data', err);
    }      
  }

  _processResize(events){    
    events = events || [];
    // filter resize events 
    let resizes = events.filter(
      item => item.event === EventTypeEnum.RESIZE);
    
    sortBy(resizes, ['ms']);
    
    if(resizes.length > 0){
      // get values from the last resize event
      let [w, h] = resizes[resizes.length-1].size.split(':');                    
      w = Number(w);
      h = Number(h);            
      this.emit('resize', { w, h });                              
    }    
  }
}

export default TtyEvents;
