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

function handleAjaxError(err){
  showError('Unable to retrieve session info');
  logger.error('fetching session length', err);
}

class TtyPlayer extends Tty {
  constructor({sid}){
    super({});
    this.sid = sid;
    this.current = 1;
    this.length = -1;
    this.ttyStream = new Array();
    this.isLoaind = false;
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
    api.get(cfg.api.getFetchSessionLengthUrl(this.sid))
      .done((data)=>{
        this.length = data.count;
        this.isReady = true;
      })
      .fail((err)=>{
        handleAjaxError(err);
        this.isError = true;
      })
      .always(()=>{
        this._change();
      });
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
      newPos = 1;
    }

    if(this.isPlaying){
      if(this.current < newPos){
        this._showChunk(this.current, newPos);
      }else{
        this.emit('reset');
        this._showChunk(this.current, newPos);
      }
    }else{
      this.current = newPos;
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
      this.current = 1;
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
    end = end + 50;
    end = end > this.length ? this.length : end;
    return api.get(cfg.api.getFetchSessionChunkUrl({sid: this.sid, start, end})).
      done((response)=>{
        for(var i = 0; i < end-start; i++){
          let {data, delay, term: {h, w}} = response.chunks[i];
          data = new Buffer(data, 'base64').toString('utf8');
          this.ttyStream[start+i] = { data, delay, w, h };
        }
      })
      .fail((err)=>{
        handleAjaxError(err);
        this.isError = true;
      })
  }

  _showChunk(start, end){
    var display = ()=>{
      for(var i = start; i < end; i++){
        this.emit('data', this.ttyStream[i].data);
      }
      this.current = end;
    };

    if(this._shouldFetch(start, end)){
      this._fetch(start, end).then(display);
    }else{
      display();
    }
  }

  _change(){
    this.emit('change');
  }
}

export default TtyPlayer;
