/*
Copyright 2018 Gravitational, Inc.

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
import * as actions from 'app/flux/fileTransfer/actions';

const withHttpRequest = httpCtor => component => {

  return class WithHttpRequestWrapper extends React.Component{

    static displayName = `WithHttpRequestWrapper`

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
        actions.updateStatus({
          id: this.fileId,
          isCompleted: true,
        })
      };

      const handleFailed = err => {
        actions.updateStatus({
          id: this.fileId,
          isFailed: true,
          error: err.message
        })
      }

      actions.updateStatus({
        id: this.fileId,
        isProcessing: true,
      })

      this.http.onProgress(handleProgress);
      this.http.onCompleted(handleCompleted);
      this.http.onError(handleFailed);
      this.http.do(this.fileUrl, this.fileBlob)
    }

    onRemove = () => {
      actions.removeFile(this.fileId);
    }

    render() {
      const { response, progress } = this.state;
      return React.createElement(component, {
        ...this.props,
        onRemove: this.onRemove,
        httpResponse: response,
        httpProgress: progress,
      });
    }
  }
}

export default withHttpRequest;