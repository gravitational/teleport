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
var cfg = require('app/config');
var {showError} = require('app/modules/notifications/actions');
var Buffer = require('buffer/').Buffer;

const logger = require('app/common/logger').create('TtyPlayer');
const STREAM_START_INDEX = 1;
const PRE_FETCH_BUF_SIZE = 5000;

function handleAjaxError(err){
  showError('Unable to retrieve session info');
  logger.error('fetching session length', err);
}

class TtyPlayer extends Tty {
  constructor({sid}){
    super({});
    this.sid = sid;
    this.current = STREAM_START_INDEX;
    this.length = -1;
    this.ttyStream = new Array();
    this.isPlaying = false;
    this.isError = false;
    this.isReady = false;
    this.isLoading = true;
  }

  send(){
  }

  resize(){
  }

  getDimensions(){
    let chunkInfo = this.ttyStream[this.current-1];
    if(chunkInfo){
       return {
         w: chunkInfo.w,
         h: chunkInfo.h
       }
    }else{
      return {w: undefined, h: undefined};
    }
  }

  connect(){
    this._setStatusFlag({isLoading: true});


    api.get(`/v1/webapi/sites/-current-/sessions/${this.sid}/events`);

    api.get(cfg.api.getFetchSessionLengthUrl(this.sid))
      .done((data)=>{
        /*
        * temporary hotfix to back-end issue related to session chunks starting at
        * index=1 and ending at index=length+1
        **/
        this.length = data.count+1;
        this._setStatusFlag({isReady: true});
      })
      .fail((err)=>{
        handleAjaxError(err);
      })
      .always(()=>{
        this._change();
      });

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

    this.timer = setInterval(this.move.bind(this), 150);
    this._change();
  }

  _shouldFetch(start, end){
    for(var i = start; i < end; i++){
      if(this.ttyStream[i] === undefined){
        return true;
      }
    }

    return false;
  }

  _fetch(start, end){
    end = end + PRE_FETCH_BUF_SIZE;
    end = end > this.length ? this.length : end;

    this._setStatusFlag({isLoading: true });

    return api.get(cfg.api.getFetchSessionChunkUrl({sid: this.sid, start, end})).
      done((response)=>{
        for(var i = 0; i < end-start; i++){
          let {data, delay, term: {h, w}} = response.chunks[i];
          data = new Buffer(data, 'base64').toString('utf8');
          this.ttyStream[start+i] = { data, delay, w, h };
        }

        this._setStatusFlag({isReady: true });
      })
      .fail((err)=>{
        handleAjaxError(err);
        this._setStatusFlag({isError: true });
      })
  }

  _display(start, end){
    let stream = this.ttyStream;
    let i;
    let tmp = [{
      data: [stream[start].data],
      w: stream[start].w,
      h: stream[start].h
    }];

    let cur = tmp[0];

    for(i = start+1; i < end; i++){
      if(cur.w === stream[i].w && cur.h === stream[i].h){
        cur.data.push(stream[i].data)
      }else{
        cur ={
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
      this.emit('resize', {h, w});
      this.emit('data', str);
    }

    this.current = end;
  }

  _showChunk(start, end){
    if(this._shouldFetch(start, end)){
      this._fetch(start, end).then(()=>
        this._display(start, end));
    }else{
      this._display(start, end);
    }
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
