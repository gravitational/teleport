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
import { Text, Flex } from 'design';
import FieldInput from 'shared/components/FieldInput';
import { ToolTipInfo } from 'shared/components/ToolTip';

import { requiredBucketName, requiredPrefixName } from './Shared/utils';

export function S3Bucket({
  s3Bucket,
  setS3Bucket,
  s3Prefix,
  setS3Prefix,
  disabled,
}: {
  s3Bucket: string;
  setS3Bucket(s: string): void;
  s3Prefix: string;
  setS3Prefix(s: string): void;
  disabled: boolean;
}) {
  return (
    <>
      <Flex alignItems="center" gap={1}>
        <Text>Amazon S3 Location</Text>
        <ToolTipInfo children="Teleport will create and use Amazon S3 Bucket as this integration's issuer and will host the required endpoints" />
      </Flex>
      <Flex gap={3}>
        <FieldInput
          rule={requiredBucketName}
          value={s3Bucket}
          placeholder="bucket"
          label="Bucket Name"
          width="50%"
          onChange={e => setS3Bucket(e.target.value.trim())}
          disabled={disabled}
        />
        <FieldInput
          rule={requiredPrefixName}
          value={s3Prefix}
          placeholder="prefix"
          label="Bucket's Prefix Name"
          width="50%"
          onChange={e => setS3Prefix(e.target.value.trim())}
          disabled={disabled}
        />
      </Flex>
    </>
  );
}
