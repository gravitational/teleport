var Tty = require('app/common/tty');
var data = Array.from(new Array(50), (x, i) => 'a' + i + '\n\n');

class TtyPlayer extends Tty {
  constructor(){
    super({});
    this.current = 0;
    this.length = data.length;
    this.isLoaind = false;
    this.isPlaying = false;
  }

  connect(){
  }

  backward(newPos){
    if(this.isPlaying){
      this.emit('reset');
    }

    this.current = newPos;
  }

  forward(newPos){
    if(this.isPlaying){
      for(;this.current < newPos; this.current++){
        this.emit('data', data[this.current]);
      }
    }

    this.current = newPos;
  }

  move(newPos){
    if(!newPos){
      newPos = this.current + 1;
    }

    if(newPos > this.length){
      newPos = this.length;
    }

    if(newPos < 0){
      newPos = 0;
    }

    if(this.current < newPos){
      this.forward(newPos);
    }else{
      this.backward(newPos);
    }

    this.current = newPos;
    this.emit('change');
  }

  stop(){
    this.isPlaying = false;
    this.timer = clearInterval(this.timer);
  }

  play(){
    if(this.isPlaying){
      return;
    }

    this.isPlaying = true;
    this.timer = setInterval(this.move.bind(this), 1000);
  }
}

export default TtyPlayer;
