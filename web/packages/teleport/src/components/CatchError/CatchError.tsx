/*
Copyright 2019-2022 Gravitational, Inc.

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
import { Failed } from 'design/CardError';
import Logger from 'shared/libs/logger';

const logger = Logger.create('components/CatchError');

export class CatchError extends React.PureComponent<Props, State> {
  state: State = { error: null };

  private retry = () => {
    this.setState({ error: null });
    this.props.onRetry?.();
  };

  static getDerivedStateFromError(error) {
    return { error };
  }

  componentDidCatch(err) {
    logger.error('render', err);
  }

  render() {
    if (this.state.error) {
      if (this.props.fallbackFn) {
        return this.props.fallbackFn({
          error: this.state.error,
          retry: this.retry,
        });
      }

      // Default fallback UI.
      return (
        <Failed alignSelf={'baseline'} message={this.state.error.message} />
      );
    }

    return this.props.children;
  }
}

type FallbackFnProp = {
  error: Error;
  retry(): void;
};

type State = {
  error: Error;
};

type Props = {
  children: React.ReactNode;
  onRetry?(): void;
  fallbackFn?(props: FallbackFnProp): React.ReactNode;
};
