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
import Logger from 'shared/libs/logger';
import api, { Signal } from 'gravity/services/api';

const logger = Logger.create('cluster/components/Logs/LogProvider');

export default class LogProvider extends React.Component {

  static propTypes = {
    queryUrl: PropTypes.string.isRequired,
    onLoading: PropTypes.func,
    onError: PropTypes.func,
    onData: PropTypes.func
  }

  constructor(props) {
    super(props)
    this._signal = null;
  }


  componentDidUpdate(prevProps) {
    if(prevProps.queryUrl !== this.props.queryUrl){
      this.fetch();
    }
  }

  componentDidMount() {
    this.fetch();
  }

  componentWillUnmount(){
    this.rejectCurrentRequest();
  }

  rejectCurrentRequest() {
    if (this._signal) {
      this._signal.abort();
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

  onData(data) {
    if (!this.props.onData) {
      return;
    }

    try {
      const parsedData = [];
      data = data || [];
      data.forEach(item => {
        item = JSON.parse(item);
        if (item.type === 'data') {
          let payload = item.payload || '';
          parsedData.push(payload.trim());
        }
      });

      if (parsedData.length === 0) {
        parsedData.push('No results found')
      }

      this.props.onData(parsedData.join('\n'));
    }catch(err){
      logger.error('Failed to deserialize', err);
    }
  }

  fetch() {
    let { queryUrl } = this.props;
    if (queryUrl) {
      queryUrl = queryUrl.trim();
    }

    this.rejectCurrentRequest();

    this.onLoading(true);

    // to abort the on-going request in situations
    // when a user executes a new query
    this._signal = new Signal();

     api.ajax( {url: queryUrl, signal: this._signal })
      .done(data => {
        this.onLoading(false);
        this.onData(data);
      })
      .fail(err => {
        // when a request is aborted by the user, its readyState is changed to
        // XMLHttpRequest.UNSENT (0) and the request's status code is set to 0.
        if (err.status === 0  && err.readyState === 0){
          // do not show any errors in this case
          return;
        }

        this.onError(err);
      });
  }

  render() {
    return null;
  }
}