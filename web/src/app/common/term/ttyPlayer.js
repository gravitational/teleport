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

var Tty = require('./tty');
var api = require('app/services/api');
var {showError} = require('app/modules/notifications/actions');
var $ = require('jQuery');
var Buffer = require('buffer/').Buffer;

const logger = require('app/common/logger').create('TtyPlayer');
const STREAM_START_INDEX = 0;
const PRE_FETCH_BUF_SIZE = 150;
const URL_PREFIX_EVENTS = '/events';
//const EVENT_MIN_TIME_DIFFERENCE = 10;
const PLAY_SPEED = 5;

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

  getCurrentEventTime(){
  }

  getLengthInTime(){
    var length = this.events.length;
    if(length === 0) {
      return 0;
    }

    return this.events[length-1].ms;
  }

  init(){
    return api.get(this.url + URL_PREFIX_EVENTS)
      .done(this._init.bind(this))
  }

  getEventsWithByteStream(start, end){
    try{
      if(this._shouldFetch(start, end)){
        // TODO: add buffering logic, as for now, load everything
        return this._fetch()
          .then(this.processByteStream.bind(this, start, this.getLength()))
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
    this.events[start].data = byteStr.slice(0, byteStrOffset).toString('utf8');
    for(var i = start+1; i < end; i++){
      let {bytes} = this.events[i];
      this.events[i].data = byteStr.slice(byteStrOffset, byteStrOffset + bytes).toString('utf8');
      byteStrOffset += bytes;
    }
  }

  _init(data){
    let {events} = data;
    let w, h;
    let tmp = [];

    // ensure that each event has the right screen size and valid values
    for(let i = 0; i < events.length; i++){

      let { ms, event, time } = events[i];

      if(event === 'resize' || event === 'session.start'){
        [w, h] = events[i].size.split(':');
      }

      if(event !== 'print'){
        continue;
      }

      // use smaller numbers
      events[i].ms = ms > 0 ? Math.floor(ms / 10) : 0;
      events[i].data = null;
      events[i].w = Number(w);
      events[i].h = Number(h);
      events[i].time = new Date(time);
      tmp.push(events[i]);
    }

    var cur = tmp[0];
    for(let i = 1; i < tmp.length; i++){
      let sameSize = cur.w === tmp[i].w && cur.h === tmp[i].h;
      let delay = tmp[i].ms - cur.ms;

      // merge events with tiny delay
      if(delay < 2 && sameSize ){
        cur.bytes += tmp[i].bytes;
        cur.ms = tmp[i].ms;
        continue;
      }

      // avoid long delays between chunks
      if(delay > 25 && delay < 50){
        tmp[i].ms = cur.ms + 25;
      }else if(delay > 50 && delay < 100){
        tmp[i].ms = cur.ms + 50;
      }else if(delay >= 100){
        tmp[i].ms = cur.ms + 100;
      }

      this.events.push(cur);
      cur = tmp[i];
    }

    if(this.events.indexOf(cur) === -1){
      this.events.push(cur);
    }
  }

  _shouldFetch(start, end){
    for(var i = start; i < end; i++){
      if(this.events[i].data === null){
        return true;
      }
    }

    return false;
  }

  _fetch(){
    let end = this.events.length - 1;
    let offset = this.events[0].offset;
    let bytes = this.events[end].offset - offset + this.events[end].bytes;
    let url = `${this.url}/stream?offset=${offset}&bytes=${bytes}`;

/*    var session = require('app/services/session');
    var { token } = session.getUserData();

    var xhr = new XMLHttpRequest();

    xhr.open('GET', encodeURI(url));
    xhr.setRequestHeader('Authorization','Bearer ' + token);
    xhr.onload = function() {
        if (xhr.status === 200) {
            alert('User\'s name is ' + xhr.responseText);
        }
        else {
            alert('Request failed.  Returned status of ' + xhr.status);
        }
    };
    xhr.send();
*/

    return api.ajax({url, processData: true, dataType: 'text' }).then((response)=>{
      return new Buffer(response);
    });
  }
}

class TtyPlayer extends Tty {
  constructor({url}){
    super({});
    this.currentEventIndex = 0;
    this.current = 0;
    this.length = -1;
    this.isPlaying = false;
    this.isError = false;
    this.isReady = false;
    this.isLoading = true;

    this._posToEventIndexMap = [];
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
        this.length = this._eventProvider.getLengthInTime();
        this._eventProvider.events.forEach(item => this._posToEventIndexMap.push(item.ms));
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

    if(newPos < 0){
      newPos = 0;
    }

    var newEventIndex = this._getEventIndex(newPos) + 1;

    if(newEventIndex === this.currentEventIndex){
      this.current = newPos;
      this._change();
      return;
    }

    try{
      let isRewind= this.currentEventIndex > newEventIndex;
      if(isRewind){
        this.emit('reset');
      }

      this._showChunk(isRewind ? 0 : this.currentEventIndex, newEventIndex)
        .then(()=>{
          this.currentEventIndex = newEventIndex;
          this.current = newPos;
          this._change();
        })
    }
    catch(err){
      logger.error('move', err);
    }
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

  _showChunk(start, end){
    this._setStatusFlag({isLoading: true });
    return this._eventProvider.getEventsWithByteStream(start, end)
      .done(events =>{
        this._setStatusFlag({isReady: true });
        this._display(events);
      })
      .fail(err=>{
        this._setStatusFlag({isError: true });
        handleAjaxError(err);
      })
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

  _setStatusFlag(newStatus){
    let {isReady=false, isError=false, isLoading=false } = newStatus;
    this.isReady = isReady;
    this.isError = isError;
    this.isLoading = isLoading;
  }

  _getEventIndex(num){
    var arr = this._posToEventIndexMap;
    var mid;
    var low = 0;
    var hi = arr.length-1;

    while (hi - low > 1) {
      mid = Math.floor ((low + hi) / 2);
      if (arr[mid] < num) {
        low = mid;
      } else {
        hi = mid;
      }
    }

    if (num - arr[low] <= arr[hi] - num) {
      return low;
    }

    return hi;
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
