var Tty = require('app/common/tty');
var api = require('app/services/api');
var cfg = require('app/config');

class TtyPlayer extends Tty {
  constructor({sid}){
    super({});
    this.sid = sid;
    this.current = 1;
    this.length = -1;
    this.ttySteam = new Array();
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

  connect(){
    api.get(cfg.api.getFetchSessionLengthUrl(this.sid))
      .done((data)=>{
        this.length = data.count;
        this.isReady = true;
      })
      .fail(()=>{
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
    }

    this.timer = setInterval(this.move.bind(this), 150);
    this._change();
  }

  _shouldFetch(start, end){
    for(var i = start; i < end; i++){
      if(this.ttySteam[i] === undefined){
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
          var data = atob(response.chunks[i].data) || '';
          var delay = response.chunks[i].delay;
          this.ttySteam[start+i] = { data, delay};
        }
      });
  }

  _showChunk(start, end){
    var display = ()=>{
      for(var i = start; i < end; i++){
        this.emit('data', this.ttySteam[i].data);
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
