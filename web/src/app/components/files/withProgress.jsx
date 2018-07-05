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

import React from 'react';

const defaultState = {
  isFailed: false,
  isProcessing: false,
  isCompleted: false,
  progress: false,
  error: ""
}

const withProgress = httpFactory => component => {

  return class WithTransferProgressWrapper extends React.Component{

    static displayName = `withTransferProgressWrapper`

    constructor(props, context) {
      super(props, context)
      this.http = new httpFactory();
      this.state = {
        ...defaultState,
        name: props.name,
      }
    }

    componentWillUnmount() {
      this.http.removeAllListeners();
      this.http.abort();
    }

    componentDidMount() {
      const handleProgress = completed => {
        this.setState({
          isProcessing: true,
          progress: completed
        })
      };

      const handleCompleted = response => {
        this.setState({
          ...defaultState,
          isCompleted: true,
          response
        })
      };

      const handleFailed = err => {
        this.setState({
          ...defaultState,
          isFailed: true,
          error: err.message
        })
      }

      const { url, blob } = this.props;
      this.http.onProgress(handleProgress);
      this.http.onCompleted(handleCompleted);
      this.http.onError(handleFailed);
      this.http.do(url, blob)
    }

    onRemove = () => {
      this.props.onRemove(this.state.name);
    }

    render() {
      const props = this.props;
      const status = this.state;
      return React.createElement(component, {
        ...props,
        status,
      });
    }
  }
}

export default withProgress;