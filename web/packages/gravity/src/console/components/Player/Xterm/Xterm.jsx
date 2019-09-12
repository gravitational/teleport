/*
Copyright 2019 Gravitational, Inc.

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
import React from 'react';
import PropTypes from 'prop-types';
import StyledXterm from './../../StyledXterm';
import XtermCtrl from 'gravity/lib/term/terminal';

export default class Xterm extends React.Component {

  static propTypes = {
    tty: PropTypes.object.isRequired
  }

  componentDidMount() {
    const tty = this.props.tty;
    this.terminal = new PlayerXtermCtrl(tty, this.refs.container);
    this.terminal.open();
  }

  componentWillUnmount() {
    this.terminal.destroy();
  }

  render() {
    const isLoading = this.props.tty.isLoading;
    // need to hide the terminal cursor while fetching for events
    const style = {
      visibility: isLoading ? "hidden" : "initial"
    }

    return (<StyledXterm p="2" style={style} ref="container" />);
  }
}

class PlayerXtermCtrl extends XtermCtrl{
  constructor(tty, el){
    super({ el, scrollBack: 1000 });
    this.tty = tty;
  }

  connect(){
  }

  open() {
    super.open();
  }

  resize(cols, rows) {
    // ensure that cursor is visible as xterm hides it on blur event
    this.term.cursorState = 1;
    super.resize(cols, rows);
  }

  destroy() {
    super.destroy();
  }

  _disconnect(){}

  _requestResize(){}
}