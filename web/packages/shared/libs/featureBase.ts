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

import { Store } from './stores';
import Logger from 'shared/libs/logger';

const logger = Logger.create('featureBase');

type Status = 'ready' | 'processing' | 'failed' | 'uninitialized' | 'disabled';

const defaultState = {
  status: 'uninitialized' as Status,
  statusText: '',
};

export default class FeatureBase extends Store<typeof defaultState> {
  state = {
    ...defaultState,
  };

  setProcessing() {
    this._setStatus('processing');
  }

  setReady() {
    this._setStatus('ready');
  }

  setDisabled() {
    this._setStatus('disabled');
  }

  setFailed(err: Error) {
    logger.error(err);
    this._setStatus('failed', err.message);
  }

  isReady() {
    return this.state.status === 'ready';
  }

  isProcessing() {
    return this.state.status === 'processing';
  }

  isFailed() {
    return this.state.status === 'failed';
  }

  isDisabled() {
    return this.state.status === 'disabled';
  }

  _setStatus(status: Status, statusText = '') {
    this.setState({ status, statusText });
  }
}

// Activator invokes onload method on a group of features.
export class Activator<T> {
  features: Loadable[];

  constructor(features: Loadable[]) {
    this.features = features || [];
  }

  onload(ctx: T) {
    this.features.forEach(f => {
      this._invokeOnload(f, ctx);
    });
  }

  _invokeOnload(f, ...props) {
    try {
      f.onload(...props);
    } catch (err) {
      logger.error('failed to invoke onload()', err);
    }
  }
}

type Loadable = {
  onload(...params: any[]): void;
};
