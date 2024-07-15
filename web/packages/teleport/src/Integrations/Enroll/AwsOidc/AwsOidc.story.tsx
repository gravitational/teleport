/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
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
