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

var React = require('react');
var cfg = require('app/config');
var session = require('app/services/session');
var Terminal = require('app/common/terminal');
var {updateSession} = require('app/modules/sessions/actions');

var TtyTerminal = React.createClass({

  componentDidMount: function() {
    let {serverId, login, sid, rows, cols} = this.props;
    let {token} = session.getUserData();
    let url = cfg.api.getTtyUrl();

    let options = {
      tty: {
        serverId, login, sid, token, url
      },
     rows,
     cols,
     el: this.refs.container
    }

    this.terminal = new Terminal(options);
    this.terminal.ttyEvents.on('data', updateSession);
    this.terminal.open();
  },

  componentWillUnmount: function() {
    this.terminal.destroy();
  },

  shouldComponentUpdate: function() {
    return false;
  },

  render() {
    return ( <div ref="container">  </div> );
  }
});

module.exports = TtyTerminal;
