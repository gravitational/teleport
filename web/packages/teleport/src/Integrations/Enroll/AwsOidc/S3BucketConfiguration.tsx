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

export function S3BucketConfiguration({
  s3Bucket,
  s3Prefix,
}: {
  s3Bucket: string;
  s3Prefix: string;
}) {
  return (
    <>
      <Flex alignItems="center" gap={1}>
        <Text>Amazon S3 Location</Text>
        <ToolTipInfo sticky={true} kind="warning">
          Deprecated. We suggest to reconfigure this integration which will
          remove the S3 bucket for you.
        </ToolTipInfo>
      </Flex>
      <Flex gap={3}>
        <FieldInput
          value={s3Bucket}
          placeholder="bucket"
          label="Bucket Name"
          width="50%"
          readonly={true}
        />
        <FieldInput
          value={s3Prefix}
          placeholder="prefix"
          label="Bucket's Prefix Name"
          width="50%"
          readonly={true}
        />
      </Flex>
    </>
  );
}
