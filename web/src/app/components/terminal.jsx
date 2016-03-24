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

var Term = require('Terminal');
var React = require('react');
var $ = require('jQuery');
var {debounce, isNumber} = require('_');
var logger = require('app/common/logger').create('components/terminal');

Term.colors[256] = '#252323';

const DISCONNECT_TXT = '\x1b[31mdisconnected\x1b[m\r\n';
const CONNECTED_TXT = 'Connected!\r\n';

var TtyTerminal = React.createClass({

  getInitialState(){
    this.rows = this.props.rows;
    this.cols = this.props.cols;
    this.tty = this.props.tty;

    this.debouncedResize = debounce(()=>{
      // tell back-end to resize
      let {cols, rows} = this._getDimensions();
      this.tty.resize(cols, rows);
    }, 200);

    return {};
  },

  componentDidMount: function() {
    this.term = new Term({
      cols: 25,
      rows: 25,
      useStyle: true,
      screenKeys: true,
      cursorBlink: true
    });

    this.term.open(this.refs.container);
    this.resize(this.cols, this.rows);

    // term events
    this.term.on('data', (data) => this.tty.send(data));

    // tty events
    this.tty.on('reset', ()=> this.term.reset());
    this.tty.on('open', ()=> this.term.write(CONNECTED_TXT));
    this.tty.on('close', ()=> this.term.write(DISCONNECT_TXT));
    this.tty.on('data', (data) => {
      try{
        this.term.write(data);
      }catch(err){
        logger.error('failed to dislpay', {data});
      }
    });

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

    let width = $container[0].clientWidth;
    let height = $container[0].clientHeight;

    let cols = Math.floor(width / (fakeColWidth));
    let rows = Math.floor(height / (fakeColHeight));
    fakeRow.remove();

    return {cols, rows};
  }

});

TtyTerminal.propTypes = {
  tty: React.PropTypes.object.isRequired
}

module.exports = TtyTerminal;
