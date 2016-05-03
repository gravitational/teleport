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

var Tty = require('app/common/tty');
var api = require('app/services/api');
var {showError} = require('app/modules/notifications/actions');
var $ = require('jQuery');

const logger = require('app/common/logger').create('TtyPlayer');
const STREAM_START_INDEX = 0;
const PRE_FETCH_BUF_SIZE = 150;
const URL_PREFIX_EVENTS = '/events';
//const EVENT_MIN_TIME_DIFFERENCE = 1;
const PLAY_SPEED = 120;

function handleAjaxError(err){
  showError('Unable to retrieve session info');
  logger.error('fetching recorded session info', err);
}

class EventProvider{
  constructor({url}){
    this.url = url;
    this.buffSize = PRE_FETCH_BUF_SIZE;
    this.events = [];
  }

  getLength(){
    return this.events.length;
  }

  init(){
    return api.get(this.url + URL_PREFIX_EVENTS)
      .done(this._init.bind(this))
  }

  getEventsWithByteStream(start, end){
    try{
      if(this._shouldFetch(start, end)){
        //simple buffering for now
        let size = this.getLength();
        let buffEnd = end + this.buffSize;
        buffEnd = buffEnd >= size ? size - 1 : buffEnd;
        return this._fetch(start, buffEnd)
          .then(this.processByteStream.bind(this, start, buffEnd))
          .then(()=> this.events.slice(start, end));
      }else{
        return $.Deferred().resolve(this.events.slice(start, end));
      }
    }catch(err){
      return $.Deferred().reject(err);
    }
  }

  processByteStream(start, end, byteStr){
    let byteStrOffset = this.events[start].bytes;
    this.events[start].data = byteStr.slice(0, byteStrOffset);

    console.info(this.events[0].data);

    for(var i = start+1; i < end; i++){
      let {bytes} = this.events[i];
      this.events[i].data = byteStr.slice(byteStrOffset, byteStrOffset + bytes);
      byteStrOffset += bytes;
      console.info(this.events[i].data);
    }
  }

  _init(data){
    let {events} = data;
    let w, h;
    let tmp = [];

    // ensure that each event has the right screen size
    for(let i = 0; i < events.length; i++){
      if(events[i].event === 'resize'){
        [w, h] = events[i].size.split(':');
      }

      if(events[i].event !== 'print'){
        continue;
      }

      events[i].data = null;
      events[i].w = Number(w);
      events[i].h = Number(h);
      //events[i].bytes = events[i].bytes || 0;
      tmp.push(events[i]);
    }

    this.events = tmp;

    // merge events that have very short time difference between each other
    /*var cur = tmp[0];
    for(let i = 1; i < tmp.length; i++){
      let sameSize = cur.w === tmp[i].w && cur.h === tmp[i].h;
      if(tmp[i].ms - cur.ms < EVENT_MIN_TIME_DIFFERENCE && sameSize ){
        cur.bytes += tmp[i].bytes;
        cur.ms = tmp[i].ms;
      }else{
        this.events.push(cur);
        cur = tmp[i];
      }
    }*/
  }

  _shouldFetch(start, end){
    for(var i = start; i < end; i++){
      if(this.events[i].data === null){
        return true;
      }
    }

    return false;
  }

  _fetch(start, end){
    let offset = this.events[start].offset;
    let bytes = this.events[end].offset - offset + this.events[end].bytes;
    let url = `${this.url}/stream?offset=${offset}&bytes=${bytes}`;

    return api.get(url).then((response)=>{
      return response.bytes;
      //return new Buffer(response.bytes, 'base64').toString('utf8');
    })
  }
}

class TtyPlayer extends Tty {
  constructor({url}){
    super({});
    this.current = STREAM_START_INDEX;
    this.length = -1;
    this.isPlaying = false;
    this.isError = false;
    this.isReady = false;
    this.isLoading = true;

    this._eventProvider = new EventProvider({url});
  }

  send(){
  }

  resize(){
  }

  connect(){
    this._setStatusFlag({isLoading: true});
    this._eventProvider.init()
      .done(()=>{
        this.length = this._eventProvider.getLength();
        this._setStatusFlag({isReady: true});
      })
      .fail(handleAjaxError)
      .always(this._change.bind(this));

    this._change();
  }

  move(newPos){
    if(!this.isReady){
      return;
    }

    if(newPos === undefined){
      newPos = this.current + 1;
    }

    if(newPos > this.length){
      newPos = this.length;
      this.stop();
    }

    if(newPos === 0){
      newPos = STREAM_START_INDEX;
    }

    if(this.current < newPos){
      this._showChunk(this.current, newPos);
    }else{
      this.emit('reset');
      this._showChunk(STREAM_START_INDEX, newPos);
    }

    this._change();
  }

  stop(){
    this.isPlaying = false;
    this.timer = clearInterval(this.timer);
    this._change();
  }

  play(){
    if(this.isPlaying){
      return;
    }

    this.isPlaying = true;

    // start from the beginning if at the end
    if(this.current === this.length){
      this.current = STREAM_START_INDEX;
      this.emit('reset');
    }

    this.timer = setInterval(this.move.bind(this), PLAY_SPEED);
    this._change();
  }

  _display(stream){
    let i;
    let tmp = [{
      data: [stream[0].data],
      w: stream[0].w,
      h: stream[0].h
    }];

    let cur = tmp[0];

    for(i = 1; i < stream.length; i++){
      if(cur.w === stream[i].w && cur.h === stream[i].h){
        cur.data.push(stream[i].data)
      }else{
        cur = {
          data: [stream[i].data],
          w: stream[i].w,
          h: stream[i].h
        };

        tmp.push(cur);
      }
    }

    for(i = 0; i < tmp.length; i ++){
      let str = tmp[i].data.join('');
      let {h, w} = tmp[i];
      if(str.length > 0){
        this.emit('resize', {h, w});
        this.emit('data', str);
      }
    }
  }

  _showChunk(start, end){
    this._setStatusFlag({isLoading: true });
    this._eventProvider.getEventsWithByteStream(start, end)
      .done(events =>{
        this._setStatusFlag({isReady: true });
        this._display(events);
        this.current = end;
      })
      .fail(err=>{
        this._setStatusFlag({isError: true });
        handleAjaxError(err);
      })
  }

  _setStatusFlag(newStatus){
    let {isReady=false, isError=false, isLoading=false } = newStatus;
    this.isReady = isReady;
    this.isError = isError;
    this.isLoading = isLoading;
  }

  _change(){
    this.emit('change');
  }
}

export default TtyPlayer;
export {
  EventProvider,
  TtyPlayer
}
