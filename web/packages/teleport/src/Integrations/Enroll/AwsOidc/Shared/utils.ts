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

import { Rule } from 'shared/components/Validation/rules';

import cfg from 'teleport/config';

// Must start and end with lowercase letters or numbers.
// Can have hyphens in between start and end.
const bucketNameRegex = new RegExp(/^[a-z0-9][a-z0-9-]*[a-z0-9]$/);
export const requiredBucketName =
  (required): Rule =>
  inputVal =>
  () => {
    if (!inputVal) {
      return {
        valid: !required,
        message: required ? 'required' : '',
      };
    }

    if (inputVal.length < 3 || inputVal.length > 63) {
      return {
        valid: false,
        message: 'name should be 3-63 characters',
      };
    }

    if (!bucketNameRegex.test(inputVal)) {
      return {
        valid: false,
        message: 'name is in a invalid format',
      };
    }

    if (inputVal.startsWith('xn--')) {
      return {
        valid: false,
        message: 'cannot start with "xn--"',
      };
    }

    if (inputVal.startsWith('sthree-')) {
      return {
        valid: false,
        message: 'cannot start with "sthree-"',
      };
    }

    if (inputVal.startsWith('sthree-configurator')) {
      return {
        valid: false,
        message: 'cannot start with "sthree-configurator"',
      };
    }

    if (inputVal.endsWith('-s3alias')) {
      return {
        valid: false,
        message: 'cannot end with "-s3alias"',
      };
    }

    if (inputVal.endsWith('--ol-s3')) {
      return {
        valid: false,
        message: 'cannot end with "--ol-s3"',
      };
    }

    return {
      valid: true,
    };
  };

// Must start and end with letters or numbers.
// Can have hyphens, underscores, and periods in between start and end.
const prefixNameRegex = new RegExp(/^[a-zA-Z0-9][a-zA-Z0-9-_.]*[a-zA-Z0-9]$/);
export const requiredPrefixName =
  (required): Rule =>
  inputVal =>
  () => {
    if (!inputVal) {
      return {
        valid: !required,
        message: required ? 'required' : '',
      };
    }

    // Just a random hard cap.
    if (inputVal.length > 63) {
      return {
        valid: false,
        message: 'name can be max 63 characters long',
      };
    }

    if (!prefixNameRegex.test(inputVal)) {
      return {
        valid: false,
        message: 'name is in a invalid format',
      };
    }

    return {
      valid: true,
    };
  };

export function getDefaultS3BucketName() {
  const modifiedClusterName = cfg.proxyCluster.replaceAll('.', '-');
  if (bucketNameRegex.test(modifiedClusterName)) {
    return modifiedClusterName;
  }

  return '';
}

export function getDefaultS3PrefixName(integrationName: string) {
  if (!integrationName || !prefixNameRegex.test(integrationName)) {
    return '';
  }

  return `${integrationName}-oidc-idp`;
}

export function validPrefixNameToolTipContent(fieldName: string) {
  return `${fieldName} name can consist only of letters and numbers. \
  Hyphens (-), dots (.), and underscores (_) are allowed in between letters and numbers.`;
}
