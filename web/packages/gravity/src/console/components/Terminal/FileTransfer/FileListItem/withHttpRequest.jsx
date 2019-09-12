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

const withHttpRequest = httpCtor => component => {

  return class WithHttpRequestWrapper extends React.Component{

    static displayName = `WithHttpRequestWrapper`

    static propTypes = {
      onUpdate: PropTypes.func.isRequired,
    }

    state = {
      progress: "0",
      response: null
    }

    constructor(props, context) {
      super(props, context)
      this.http = new httpCtor();
      this.fileId = props.file.id;
      this.fileBlob = props.file.blob;
      this.fileUrl = props.file.url;
    }

    componentWillUnmount() {
      this.http.removeAllListeners();
      this.http.abort();
    }

    componentDidMount() {
      const handleProgress = completed => {
        this.setState({ progress: completed })
      };

      const handleCompleted = response => {
        this.state.response = response;
        this.props.onUpdate({
          id: this.fileId,
          isCompleted: true,
        })
      };

      const handleFailed = err => {
        this.props.onUpdate({
          id: this.fileId,
          isFailed: true,
          error: err.message
        })
      }

      this.props.onUpdate({
        id: this.fileId,
        isProcessing: true,
      })

      this.http.onProgress(handleProgress);
      this.http.onCompleted(handleCompleted);
      this.http.onError(handleFailed);
      this.http.do(this.fileUrl, this.fileBlob)
    }

    render() {
      const { response, progress } = this.state;
      return React.createElement(component, {
        ...this.props,
        httpResponse: response,
        httpProgress: progress,
      });
    }
  }
}

export default withHttpRequest;