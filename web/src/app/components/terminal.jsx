var Term = require('Terminal');
var React = require('react');
var {debounce, isNumber} = require('_');

Term.colors[256] = 'inherit';

const DISCONNECT_TXT = '\x1b[31mdisconnected\x1b[m\r\n';
const CONNECTED_TXT = 'Connected!\r\n';

var TtyTerminal = React.createClass({

  getInitialState(){
    this.rows = this.props.rows;
    this.cols = this.props.cols;
    this.tty = this.props.tty;

    this.debouncedResize = debounce(()=>{
      this.resize();
      this.tty.resize(this.cols, this.rows);
    }, 200);

    return {};
  },

  componentDidMount: function() {
    this.term = new Terminal({
      cols: 5,
      rows: 5,
      useStyle: true,
      screenKeys: true,
      cursorBlink: true
    });

    this.term.open(this.refs.container);
    this.term.on('data', (data) => this.tty.send(data));

    this.resize(this.cols, this.rows);

    this.tty.on('open', ()=> this.term.write(CONNECTED_TXT));
    this.tty.on('close', ()=> this.term.write(DISCONNECT_TXT));
    this.tty.on('data', (data) => this.term.write(data));
    this.tty.on('reset', ()=> this.term.reset());

    this.tty.connect({cols: this.cols, rows: this.rows});
    window.addEventListener('resize', this.debouncedResize);
  },

  componentWillUnmount: function() {
    this.term.destroy();
    window.removeEventListener('resize', this.debouncedResize);
  },

  shouldComponentUpdate: function(newProps) {
    var {rows, cols} = newProps;

    if( !isNumber(rows) || !isNumber(cols)){
      return false;
    }

    if(rows !== this.rows || cols !== this.cols){
      this.resize(cols, rows)
    }

    return false;
  },

  render() {
    return ( <div className="grv-terminal" id="terminal-box" ref="container">  </div> );
  },

  resize: function(cols, rows) {
    // if not defined, use the size of the container
    if(!isNumber(cols) || !isNumber(rows)){
      let dim = this._getDimensions();
      cols = dim.cols;
      rows = dim.rows;
    }

    this.cols = cols;
    this.rows = rows;

    this.term.resize(this.cols, this.rows);
  },

  _getDimensions(){
    let $container = $(this.refs.container);
    let fakeRow = $('<div><span>&nbsp;</span></div>');

    $container.find('.terminal').append(fakeRow);
    // get div height
    let fakeColHeight = fakeRow[0].getBoundingClientRect().height;
    // get span width
    let fakeColWidth = fakeRow.children().first()[0].getBoundingClientRect().width;
    let cols = Math.floor($container.width() / (fakeColWidth));
    let rows = Math.floor($container.height() / (fakeColHeight));
    fakeRow.remove();

    return {cols, rows};
  }

});

TtyTerminal.propTypes = {
  tty: React.PropTypes.object.isRequired
}

module.exports = TtyTerminal;
