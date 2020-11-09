/**
 * Copyright 2020 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import { RequestReason } from './RequestReason';

export default {
  title: 'Teleport/AccessStrategy/Reason',
};

export const Loaded = () => {
  return <RequestReason {...sample} />;
};

export const Processing = () => {
  const attempt = {
    isProcessing: true,
    isFailed: false,
    isSuccess: false,
    message: '',
  };
  return <RequestReason {...sample} attempt={attempt} />;
};

export const Failed = () => {
  const attempt = {
    isProcessing: false,
    isFailed: true,
    isSuccess: false,
    message: 'some error message',
  };

  return <RequestReason {...sample} attempt={attempt} />;
};

export const LoadedWithPrompt = () => {
  return (
    <RequestReason
      {...sample}
      prompt={'Some custom prompt set by administrator'}
    />
  );
};

const sample = {
  attempt: {
    isProcessing: false,
    isFailed: false,
    isSuccess: false,
    message: '',
  },
  prompt: '',
  reason: 'some reason',
  setReason: () => null,
  createRequest: () => Promise.resolve(null),
};
