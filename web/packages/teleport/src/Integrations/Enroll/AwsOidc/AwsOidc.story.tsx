/**
 * Copyright 2023 Gravitational, Inc.
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
import { MemoryRouter } from 'react-router';

import { AwsOidc } from './AwsOidc';
import { S3BucketWarningBanner } from './S3BucketWarningBanner';

export default {
  title: 'Teleport/Integrations/Enroll/AwsOidc',
};

export const Flow = () => (
  <MemoryRouter>
    <AwsOidc />
  </MemoryRouter>
);

export const SBucketWarning = () => (
  <S3BucketWarningBanner onClose={() => null} onContinue={() => null} />
);

export const SBucketWarningWithReview = () => (
  <S3BucketWarningBanner
    onClose={() => null}
    onContinue={() => null}
    reviewing={true}
  />
);
