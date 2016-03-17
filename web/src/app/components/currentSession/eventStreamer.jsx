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

var cfg = require('app/config');
var React = require('react');
var session = require('app/services/session');
var {updateSession} = require('app/modules/sessions/actions');

var EventStreamer = React.createClass({
  componentDidMount() {
    let {sid} = this.props;
    let {token} = session.getUserData();
    let connStr = cfg.api.getEventStreamConnStr(token, sid);

    this.socket = new WebSocket(connStr, 'proto');
    this.socket.onmessage = (event) => {
      try
      {
        let json = JSON.parse(event.data);
        updateSession(json.session);
      }
      catch(err){
        console.log('failed to parse event stream data');
      }

    };
    this.socket.onclose = () => {};
  },

  componentWillUnmount() {
    this.socket.close();
  },

  shouldComponentUpdate() {
    return false;
  },

  render() {
    return null;
  }
});

export default EventStreamer;
