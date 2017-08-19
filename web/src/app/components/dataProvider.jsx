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

import { Component } from 'react';

const DEFAULT_INTERVAL = 3000; // every 3 sec

export default class DataProvider extends Component {
  
  _timerId = null;
  _request = null;

  constructor(props) {
    super(props);
    this._intervalTime = props.time || DEFAULT_INTERVAL;
  }

  fetch() {
    // do not refetch if still in progress
    if (this._request) {      
      return;
    }

    this._request = this.props.onFetch()
      .always(() => {
        this._request = null;
      })
  }

  componentDidMount() {        
    this.fetch();
    this._timerId = setInterval(this.fetch.bind(this), this._intervalTime);
  }

  componentWillUnmount(){
    clearInterval(this._timerId);
  }

  render() {
    return null;
  }
}