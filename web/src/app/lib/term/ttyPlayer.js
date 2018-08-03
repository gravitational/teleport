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
import { EventTypeEnum, TermEventEnum } from './enums';
import Logger from 'app/lib/logger';

const logger = Logger.create('TtyPlayer');
const STREAM_START_INDEX = 0;
const URL_PREFIX_EVENTS = '/events';
const PLAY_SPEED = 5;
const Buffer = BufferModule.Buffer;

export const MAX_SIZE = 5242880; // 5mg

export class EventProvider{
  constructor({url}){
    this.url = url;
    this.events = [];
  }

  getDuration() {
    const eventCount = this.events.length;
    if(eventCount === 0) {
      return 0;
    }

    return this.events[eventCount-1].msNormalized;
  }

  init() {
    return this._fetchEvents().then(events => {
      this.events = events;
      const printEvents = this.events.filter(onlyPrintEvents);
      if (printEvents.length === 0) {
        return;
      }

      return this._fetchContent(printEvents).then(buffer => {
        this._populatePrintEvents(buffer, printEvents);
      });
    });
  }

  _fetchEvents() {
    const url = this.url + URL_PREFIX_EVENTS;
    return api.get(url).then(json => {
      if (!json.events) {
        return [];
      }

      return this._createEvents(json.events);
    })
  }

  _fetchContent(events) {
    // calclulate the size of the session in bytes to know how many
    // chunks to load due to maximum chunk size.
    let offset = events[0].offset;
    const end = events.length - 1;
    const totalSize = events[end].offset - offset + events[end].bytes;
    const chunkCount = Math.ceil(totalSize / MAX_SIZE);

    // create a fetch request for each chunk
    const promises = [];
    for (let i = 0; i < chunkCount; i++){
      const url = `${this.url}/stream?offset=${offset}&bytes=${MAX_SIZE}`;
      promises.push(api.ajax({
        url,
        processData: true,
        dataType: 'text'
      }));

      offset = offset + MAX_SIZE;
    }

    // fetch all session chunks and then merge them in one
    return $.when(...promises)
      .then((...responses) => {
        responses = promises.length === 1 ? [[responses]] : responses;
        const allBytes = responses.reduce((byteStr, r) => byteStr + r[0], '');
        return new Buffer(allBytes);
      });
  }

  _populatePrintEvents(buffer, events){
    let byteStrOffset = events[0].bytes;
    events[0].data = buffer.slice(0, byteStrOffset).toString('utf8');
    for(var i = 1; i < events.length; i++){
      let {bytes} = events[i];
      events[i].data = buffer.slice(byteStrOffset, byteStrOffset + bytes).toString('utf8');
      byteStrOffset += bytes;
    }
  }

  _createEvents(json) {
    let w, h;
    let events = [];

    // filter print events and ensure that each has the right screen size and valid values
    for(let i = 0; i < json.length; i++){
      const { ms, event, offset, time, bytes } = json[i];

      // grab new screen size for the next events
      if(event === EventTypeEnum.RESIZE || event === EventTypeEnum.START){
        [w, h] = json[i].size.split(':');
      }

      // session has ended, stop here
      if (event === EventTypeEnum.END) {
        const start = new Date(events[0].time);
        const end = new Date(time);
        const duration = end.getTime() - start.getTime();
        const displayTime = this._formatDisplayTime(duration);
        events.push({
          eventType: event,
          displayTime,
          ms: duration,
          time: new Date(time)
        });

        break;
      }

      // process only PRINT events
      if(event !== EventTypeEnum.PRINT){
        continue;
      }

      let displayTime = this._formatDisplayTime(ms);

      events.push({
        eventType: EventTypeEnum.PRINT,
        displayTime,
        ms,
        bytes,
        offset,
        data: null,
        w: Number(w),
        h: Number(h),
        time: new Date(time)
      });
    }

    return this._normalizeEventsByTime(events);
  }

  _normalizeEventsByTime(events) {
    if (!events || events.length === 0) {
      return [];
    }

    events.forEach(e => {
      e.ms = e.ms > 0 ? Math.floor(e.ms / 10) : 0;
      e.msNormalized = e.ms;
    })

    let cur = events[0];
    let tmp = [];
    for (let i = 1; i < events.length; i++){
      const sameSize = cur.w === events[i].w && cur.h === events[i].h;
      const delay = events[i].ms - cur.ms;

      // merge events with tiny delay
      if(delay < 2 && sameSize ){
        cur.bytes += events[i].bytes;
        continue;
      }

      // avoid long delays between chunks
      events[i].msNormalized = cur.msNormalized + shortenTime(delay);

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

function shortenTime(value) {
  if (value >= 25 && value < 50) {
    return 25;
  } else if (value >= 50 && value < 100) {
    return  50;
  } else if (value >= 100) {
    return 100;
  } else {
    return value;
  }
}

function onlyPrintEvents(e) {
  return e.eventType === EventTypeEnum.PRINT;
}

export class TtyPlayer extends Tty {
  constructor({url}){
    super({});
    this.currentEventIndex = 0;
    this.current = 0;
    this.duration = 0;
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
    this.duration = this._eventProvider.getDuration();
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

    if(newPos < 0){
      newPos = 0;
    }

    if(newPos > this.duration){
      this.stop();
    }

    const newEventIndex = this._getEventIndex(newPos) + 1;

    if(newEventIndex === this.currentEventIndex){
      this.current = newPos;
      this._change();
      return;
    }

    const isRewind = this.currentEventIndex > newEventIndex;

    try {
      // we cannot playback the content within terminal so instead:
      // 1. tell terminal to reset.
      // 2. tell terminal to render 1 huge chunk that has everything up to current
      // location.
      if (isRewind) {
        this.emit(TermEventEnum.RESET);
      }

      const from = isRewind ? 0 : this.currentEventIndex;
      const to = newEventIndex;
      const events = this._eventProvider.events.slice(from, to);
      const printEvents = events.filter(onlyPrintEvents);

      this._display(printEvents);
      this.currentEventIndex = newEventIndex;
      this.current = newPos;
      this._change();
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
    if(this.current >= this.duration){
      this.current = STREAM_START_INDEX;
      this.emit(TermEventEnum.RESET);
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

  _display(events) {
    if (!events || events.length === 0) {
      return;
    }

    const groups = [{
      data: [events[0].data],
      w: events[0].w,
      h: events[0].h
    }];

    let cur = groups[0];

    // group events by screen size and construct 1 chunk of data per group
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
        this.emit(TermEventEnum.RESIZE, { h, w });
        this.emit(TermEventEnum.DATA, str);
      }
    }
  }

  _setStatusFlag(newStatus){
    const {
      isReady = false,
      isError = false,
      isLoading = false,
      errText = '' } = newStatus;

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

/* const mamaData = atob('cm9vdEB0MS1tYXN0ZXI6fiMgDRtbS3Jvb3RAdDEtbWFzdGVyOn4jIA==');

const mamaEvents = [{
      "addr.local": "127.0.0.1:3022",
      "addr.remote": "xxx.xxx.xxx.xxx:47452",
      "ei": 0,
      "event": "session.start",
      "id": 0,
      "login": "root",
      "namespace": "default",
      "server_id": "5cd9de35-3432-4926-af05-c326b5bb8329",
      "sid": "d30ae7e7-92b4-11e8-93f5-525400432101",
      "size": "80:25",
      "time": "2018-07-28T22:23:17.502Z",
      "user": "alex-kovoy"
  }, {
      "bytes": 18,
      "ci": 0,
      "ei": 1,
      "event": "print",
      "id": 1,
      "ms": 0,
      "offset": 0,
      "time": "2018-07-28T22:23:17.518Z"
  }, {
      "ei": 2,
      "event": "resize",
      "id": 2,
      "login": "root",
      "namespace": "default",
      "sid": "d30ae7e7-92b4-11e8-93f5-525400432101",
      "size": "162:62",
      "time": "2018-07-28T22:23:17.536Z",
      "user": "alex-kovoy"
  }, {
      "bytes": 22,
      "ci": 1,
      "ei": 3,
      "event": "print",
      "id": 3,
      "ms": 19,
      "offset": 18,
      "time": "2018-07-28T22:23:17.537Z"
  }, {
      "ei": 4,
      "event": "session.leave",
      "id": 4,
      "namespace": "default",
      "server_id": "5cd9de35-3432-4926-af05-c326b5bb8329",
      "sid": "d30ae7e7-92b4-11e8-93f5-525400432101",
      "time": "2018-07-28T22:23:42.972Z",
      "user": "alex-kovoy"
  }, {
      "ei": 5,
      "event": "session.end",
      "id": 5,
      "namespace": "default",
      "sid": "d30ae7e7-92b4-11e8-93f5-525400432101",
      "time": "2018-07-28T22:24:02.973Z",
      "user": "alex-kovoy"
  }]
 */