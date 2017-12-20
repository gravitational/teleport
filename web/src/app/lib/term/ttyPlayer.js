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

import $ from 'jQuery';
import BufferModule from 'buffer/';
import api from 'app/services/api';
import Tty from './tty';
import { EventTypeEnum } from './enums';
import Logger from 'app/lib/logger';

const logger = Logger.create('TtyPlayer');
const STREAM_START_INDEX = 0;
const PRE_FETCH_BUF_SIZE = 150;
const URL_PREFIX_EVENTS = '/events';
const PLAY_SPEED = 5;

const Buffer = BufferModule.Buffer;

export class EventProvider{
  constructor({url}){
    this.url = url;
    this.buffSize = PRE_FETCH_BUF_SIZE;
    this.events = [];
  }
    
  getLengthInTime(){
    var length = this.events.length;
    if(length === 0) {
      return 0;
    }

    return this.events[length-1].msNormalized;
  }

  init(){
    return api.get(this.url + URL_PREFIX_EVENTS).done(json => {
      if (!json.events) {
        return;
      }

      let events = this._createPrintEvents(json.events);
      events = this._normalizeEventsByTime(events);
      this.events = events;        
    });
  }

  hasAll(start, end){
    for(var i = start; i < end; i++){
      if(this.events[i].data === null){
        return false;
      }
    }

    return true;
  }

  sliceEvents(start, end) {
    return this.events.slice(start, end);
  }

  fetchEvents(start, end){    
    // TODO: uncomment it once partial fetch is implemented on the backend
    return this._fetch(/*start, end*/)
      .then(this.processByteStream.bind(this, start, this.events.length))
      .then(()=> this.events.slice(start, end));          
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
  
  _fetch(){
    const end = this.events.length - 1;
    const offset = this.events[0].offset;
    const bytes = this.events[end].offset - offset + this.events[end].bytes;
    const url = `${this.url}/stream?offset=${offset}&bytes=${bytes}`;
    return api.ajax({ url, processData: true, dataType: 'text' }).then(response => {                  
      return new Buffer(response);
    });
  }
  
  _createPrintEvents(json) {          
    let w, h;
    let events = [];

    // filter print events and ensure that each has the right screen size and valid values
    for(let i = 0; i < json.length; i++){

      let { ms, event, offset, time, bytes } = json[i];

      // grab new screen size for the next events
      if(event === EventTypeEnum.RESIZE || event === EventTypeEnum.START){
        [w, h] = json[i].size.split(':');
      }
      
      // session has ended, stop here
      if (event === EventTypeEnum.END) {
        break;
      }

      // process only PRINT events      
      if(event !== EventTypeEnum.PRINT){
        continue;
      }

      let displayTime = this._formatDisplayTime(ms);

      // use smaller numbers
      ms =  ms > 0 ? Math.floor(ms / 10) : 0;

      events.push({
        displayTime,
        ms,
        msNormalized: ms,
        bytes,
        offset,
        data: null,
        w: Number(w),
        h: Number(h),
        time: new Date(time)
      });      
    }

    return events;
  }

  _normalizeEventsByTime(events){    
    let cur = events[0];
    let tmp = [];
    for(let i = 1; i < events.length; i++){
      let sameSize = cur.w === events[i].w && cur.h === events[i].h;
      let delay = events[i].ms - cur.ms;

      // merge events with tiny delay
      if(delay < 2 && sameSize ){
        cur.bytes += events[i].bytes;
        cur.msNormalized += delay;
        continue;
      }

      // avoid long delays between chunks
      if(delay >= 25 && delay < 50){
        events[i].msNormalized = cur.msNormalized + 25;
      }else if(delay >= 50 && delay < 100){
        events[i].msNormalized = cur.msNormalized + 50;
      }else if(delay >= 100){
        events[i].msNormalized = cur.msNormalized + 100;
      }else{
        events[i].msNormalized = cur.msNormalized + delay;
      }

      tmp.push(cur);
      cur = events[i];
    }

    if(tmp.indexOf(cur) === -1){
      tmp.push(cur);
    }

    return tmp;
  }

  _formatDisplayTime(ms){
    if(ms < 0){
      return '00:00';
    }

    let totalSec = Math.floor(ms / 1000);
    let totalDays = (totalSec % 31536000) % 86400;
    let h = Math.floor(totalDays / 3600);
    let m = Math.floor((totalDays % 3600) / 60);
    let s = (totalDays % 3600) % 60;

    m = m > 9 ? m : '0' + m;
    s = s > 9 ? s : '0' + s;
    h = h > 0 ? h + ':' : '';

    return `${h}${m}:${s}`;
  }

}

export class TtyPlayer extends Tty {
  constructor({url}){
    super({});
    this.currentEventIndex = 0;
    this.current = 0;
    this.length = -1;
    this.isPlaying = false;
    this.isError = false;
    this.isReady = false;
    this.isLoading = true;
    this.errText = '';

    this._posToEventIndexMap = [];
    this._eventProvider = new EventProvider({url});
  }

  // override
  send(){
  }

  // override
  resize(){
  }

  // override
  connect(){
    this._setStatusFlag({isLoading: true});
    this._eventProvider.init()
      .then(() => {
        this._init();
        this._setStatusFlag({isReady: true});
      })
      .fail(err => {
        logger.error('unable to init event provider', err);
        this.handleError(err);        
      })
      .always(this._change.bind(this));

    this._change();
  }

  handleError(err) {
    this._setStatusFlag({
      isError: true,
      errText: api.getErrorText(err)
    })    
  }

  _init(){
    this.length = this._eventProvider.getLengthInTime();
    this._eventProvider.events.forEach(item =>
      this._posToEventIndexMap.push(item.msNormalized));    
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

    const newEventIndex = this._getEventIndex(newPos) + 1;

    if(newEventIndex === this.currentEventIndex){
      this.current = newPos;
      this._change();
      return;
    }

    const isRewind = this.currentEventIndex > newEventIndex;
    
    try {            

      // we cannot playback the content within terminal so instead we do this:
      // 1. tell terminal to reset.
      // 2. tell terminal to render 1 huge chunk that has everything up to current
      // newEventIndex.
      if (isRewind) {        
        this.emit('reset');
      }

      this._showChunk(isRewind ? 0 : this.currentEventIndex, newEventIndex)
        .then(()=>{
          this.currentEventIndex = newEventIndex;
          this.current = newPos;
          this._change();
        })
        .fail(err => {
          logger.error('unable to process a chunk of session recording', err);
          this.handleError(err);        
        })
    }
    catch(err){
      logger.error('move', err);
      this.handleError(err);        
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

  getCurrentTime(){
    if(this.currentEventIndex){
      let {displayTime} = this._eventProvider.events[this.currentEventIndex-1];
      return displayTime;
    }else{
      return '--:--';
    }
  }

  getEventCount() {
    return this._eventProvider.events.length;
  }

  _showChunk(start, end) {          
    // check if all events exist within given interval
    if (this._eventProvider.hasAll(start, end)) {
      const events = this._eventProvider.sliceEvents(start, end);      
      this._display(events);
      return $.Deferred().resolve();
    } 

    // fetch requests within given interval
    this._setStatusFlag({ isLoading: true });
    return this._eventProvider.fetchEvents(start, end).done(events => {
      this._setStatusFlag({ isReady: true });
      this._display(events);
    });
  }

  _display(events) {        
    const groups = [{
      data: [events[0].data],
      w: events[0].w,
      h: events[0].h
    }];

    let cur = groups[0];
    
    // group events based on screen size and merge by creating 1 event per each screen size
    for(let i = 1; i < events.length; i++){
      if(cur.w === events[i].w && cur.h === events[i].h){
        cur.data.push(events[i].data)
      }else{
        cur = {
          data: [events[i].data],
          w: events[i].w,
          h: events[i].h
        };

        groups.push(cur);
      }
    }

    // render each group
    for(let i = 0; i < groups.length; i ++){
      const str = groups[i].data.join('');
      const {h, w} = groups[i];
      if (str.length > 0) {                        
        this.emit('resize', { h, w });                
        this.emit('data', str);        
      }
    }
  }

  _setStatusFlag(newStatus){
    const { isReady=false, isError=false, isLoading=false, errText='' } = newStatus;
    this.isReady = isReady;
    this.isError = isError;
    this.isLoading = isLoading;
    this.errText = errText;
  }

  _getEventIndex(num){
    const arr = this._posToEventIndexMap;    
    var low = 0;
    var hi = arr.length - 1;
    
    while (hi - low > 1) {
      const mid = Math.floor ((low + hi) / 2);
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
export { Buffer } 