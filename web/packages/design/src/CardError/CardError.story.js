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
import * as CardError from './CardError';

const message = 'some error message';

export default {
  title: 'Design/CardError',
};

export const NotFound = () => <CardError.NotFound message={message} />;

export const AccessDenied = () => <CardError.AccessDenied message={message} />;

export const Failed = () => <CardError.Failed message={message} />;

export const LoginFailed = () => (
  <CardError.LoginFailed message={message} loginUrl="https://localhost" />
);

export const Offline = () => (
  <CardError.Offline
    title={'This cluster is not available from Gravity.'}
    message={'To access this cluster, please use its local endpoint'}
  />
);
