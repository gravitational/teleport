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

export const AWS_IAM_ARN_DEFAULT_PARTITION = 'arn:aws:iam::';
export const AWS_IAM_ARN_CHINA_PARTITION = 'arn:aws-cn:iam::';
export const AWS_IAM_ARN_USGOV_PARTITION = 'arn:aws-us-gov:iam::';

/**
 * ROLE_ARN_REGEX uses the same regex matcher used in the backend:
 * https://github.com/gravitational/teleport/blob/2cba82cb332e769ebc8a658d32ff24ddda79daff/api/utils/aws/identifiers.go#L43
 *
 * The regex checks for alphanumerics and select few characters.
 */
export const IAM_ROLE_NAME_REGEX = /^[\w+=,.@-]+$/;

export const IAM_ROLE_ARN_REGEX =
  /^arn:(aws|aws-cn|aws-us-gov):iam::\d{12}:role\/[\w+=,.@-]+$/;

/**
 * @returns
 *   - awsAccountId: the 12 digit aws account Id
 *   - arnStartingPart: starting part is returned in the format
 *    "arn:\<partition\>:iam::"
 *   - arnResourceName: is the resource name in the resource part
 *     of arn: <resource-type>/<this-user-defined-resource-name>
 */
export function splitAwsIamArn(arn: string): {
  awsAccountId: string;
  arnStartingPart: string;
  arnResourceName: string;
} {
  if (!arn) {
    return {
      awsAccountId: '',
      arnStartingPart: '',
      arnResourceName: '',
    };
  }

  let awsAccountId: string;
  let arnStartingPart: string;
  let splitted: string[] = [];

  if (arn.startsWith(AWS_IAM_ARN_DEFAULT_PARTITION)) {
    arnStartingPart = AWS_IAM_ARN_DEFAULT_PARTITION;
    splitted = arn.split(AWS_IAM_ARN_DEFAULT_PARTITION);
  } else if (arn.startsWith(AWS_IAM_ARN_CHINA_PARTITION)) {
    arnStartingPart = AWS_IAM_ARN_CHINA_PARTITION;
    splitted = arn.split(AWS_IAM_ARN_CHINA_PARTITION);
  } else if (arn.startsWith(AWS_IAM_ARN_USGOV_PARTITION)) {
    arnStartingPart = AWS_IAM_ARN_USGOV_PARTITION;
    splitted = arn.split(AWS_IAM_ARN_USGOV_PARTITION);
  }

  awsAccountId = splitted[1]?.substring(0, 12) ?? '';

  return {
    awsAccountId,
    arnStartingPart,
    arnResourceName: splitted[1]?.substring(12).replace(/^(:role\/)/, ''),
  };
}
