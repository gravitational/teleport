/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import { splitAwsIamArn } from './aws';

describe('splitAwsIamArn', () => {
  it.each([
    {
      name: 'with default partition',
      arn: 'arn:aws:iam::123456789012:role/johndoe',
      expected: {
        awsAccountId: '123456789012',
        arnStartingPart: 'arn:aws:iam::',
        arnResourceName: 'johndoe',
      },
    },
    {
      name: 'with china partition',
      arn: 'arn:aws-cn:iam::123456789012:role/johndoe',
      expected: {
        awsAccountId: '123456789012',
        arnStartingPart: 'arn:aws-cn:iam::',
        arnResourceName: 'johndoe',
      },
    },
    {
      name: 'with us gov partition',
      arn: 'arn:aws-us-gov:iam::123456789012:role/johndoe',
      expected: {
        awsAccountId: '123456789012',
        arnStartingPart: 'arn:aws-us-gov:iam::',
        arnResourceName: 'johndoe',
      },
    },
  ])('$name', ({ arn, expected }) => {
    const result = splitAwsIamArn(arn);

    expect(result).toStrictEqual(expected);
  });
});
