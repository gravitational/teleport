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

import { Flex, Text } from 'design';
import { IconTooltip } from 'design/Tooltip';
import FieldInput from 'shared/components/FieldInput';

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
        <IconTooltip kind="warning">
          Deprecated. Amazon is now validating the IdP certificate against a
          list of root CAs. Storing the OpenID Configuration in S3 is no longer
          required, and should be removed to improve security.
        </IconTooltip>
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
