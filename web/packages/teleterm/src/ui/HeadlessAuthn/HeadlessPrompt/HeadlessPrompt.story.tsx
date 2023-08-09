/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';

import { makeEmptyAttempt } from 'shared/hooks/useAsync';

import { makeRootCluster } from 'teleterm/services/tshd/testHelpers';

import { HeadlessPrompt } from './HeadlessPrompt';

export default {
  title: 'Teleterm/ModalsHost/HeadlessPrompt',
};

export const Story = () => (
  <HeadlessPrompt
    cluster={makeRootCluster()}
    clientIp="localhost"
    skipConfirm={false}
    onApprove={async () => {}}
    abortApproval={() => {}}
    onReject={async () => {}}
    updateHeadlessStateAttempt={makeEmptyAttempt<void>()}
    onCancel={() => {}}
    headlessAuthenticationId="85fa45fa-57f4-5a9d-9ba8-b3cbf76d5ea2"
  />
);
