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
import { getAccessToken } from 'gravity/services/api';
import cfg from 'gravity/config';
import { generatePath } from 'react-router';

export default class InstallLogsProvider extends React.Component {

  static propTypes = {
   siteId: PropTypes.string.isRequired,
   opId: PropTypes.string.isRequired,
   onLoading: PropTypes.func,
   onError: PropTypes.func,
   onData: PropTypes.func
  }

  constructor(props) {
    super(props);
    this.socket = null;
  }

  componentWillReceiveProps(nextProps){
    let {siteId, opId} = this.props;
    if(nextProps.opId !== opId){
      this.connect(siteId, nextProps.opId);
    }
  }

  componentDidMount() {
    let {siteId, opId} = this.props;
    this.connect(siteId, opId);
  }

  componentWillUnmount(){
    this.disconnect();
  }

  disconnect(){
    if(this.socket){
      this.socket.close();
    }
  }

  onLoading(value){
    if(this.props.onLoading){
      this.props.onLoading(value);
    }
  }

  onError(err){
    if(this.props.onError){
      this.props.onError(err);
    }
  }

  onData(data){
    if(this.props.onData){
      this.props.onData(data.trim() + '\n');
    }
  }

  connect(siteId, opId){
    this.disconnect();
    this.onLoading(true);

    this.socket = createLogStreamer(siteId, opId);
    this.socket.onopen = () => { this.onLoading(false); };
    this.socket.onerror = () => { this.onError(); }
    this.socket.onclose = () => { };
    this.socket.onmessage = e => { this.onData(e.data); };
  }

  render() {
     return null;
  }
}

function createLogStreamer(siteId, opId){
  const token = getAccessToken();
  const hostport = location.hostname + (location.port ? ':' + location.port : '');
  const hostname = `wss://${hostport}`;
  const url = generatePath(cfg.api.operationLogsPath, {
    siteId,
    token,
    opId
  });

  return new WebSocket(hostname + url);
}