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

import {
  Rule,
  ValidationResult,
  validAwsIAMRoleName,
} from 'shared/components/Validation/rules';

import { awsRegionMap } from 'teleport/services/integrations';

/**
 * requiredAwsIdentityCenterRegion checks if the provided AWS Identity
 * region name is an officially supported AWS region.
 * @param region ARN of AWS IAM Identity Center instance.
 * @returns ValidationResult
 */
export const requiredAwsIdentityCenterRegion: Rule =
  region => (): ValidationResult => {
    if (!region) {
      return {
        valid: false,
        message: 'AWS IAM Identity Center region is required',
      };
    }

    const officialRegions = Object.keys(awsRegionMap);
    if (!officialRegions.includes(region)) {
      return {
        valid: false,
        message: 'AWS IAM Identity Center region is not valid',
      };
    }

    return { valid: true };
  };

/**
 * requiredAwsIdentityCenterInstanceArn checks if the provided AWS Identity
 * center instance ARN is of a valid format.
 * Reference format: arn:aws:sso:::instance/ssoins-99aa88aa22iiss99
 * @param instanceArn ARN of AWS IAM Identity Center instance.
 * @returns ValidationResult
 */
export const requiredAwsIdentityCenterInstanceArn: Rule =
  instanceArn => (): ValidationResult => {
    if (!instanceArn) {
      return {
        valid: false,
        message: 'AWS IAM Identity Center instance ARN is required',
      };
    }
    const regex = new RegExp('^arn:aws.*:sso:::instance/ssoins-*');

    if (regex.test(instanceArn)) {
      return {
        valid: true,
      };
    }

    return {
      valid: false,
      message:
        'Invalid instance ARN, double check you copied and pasted the correct output',
    };
  };

/**
 * requiredOidcIntegrationName is a required field and checks for a
 * value which should also be a valid AWS IAM role name.
 * @param name is an integration name.
 */
export const requiredOidcIntegrationName: Rule =
  name => (): ValidationResult => {
    if (!name) {
      return {
        valid: false,
        message: 'Integration name required',
      };
    }

    return validAwsIAMRoleName(name);
  };

/**
 * requireUniqueIntegrationName checks if the provided integration
 * name already exists in Teleport.
 * @param awsIntegrationNames array of existing integration name.
 * @param name integration name input.
 * @returns ValidationResult
 */
export const requireUniqueIntegrationName =
  (awsIntegrationNames: string[]) => (name: string) => () => {
    if (awsIntegrationNames === null) {
      return {
        valid: false,
        message: 'Failed to fetch existing AWS OIDC integrations',
      };
    }
    if (awsIntegrationNames.includes(name)) {
      return {
        valid: false,
        message: 'Integration name already exists',
      };
    }
    return {
      valid: true,
    };
  };

/**
 * requiredHttpsUrl validates if input is a valid HTTPs endpoint.
 * @param urlInput is a url
 * @returns ValidationResult
 */
export const requiredHttpsUrl: Rule = urlInput => () => {
  if (!urlInput) {
    return {
      valid: false,
      message: 'SCIM endpoint is required',
    };
  }

  let url: URL;
  try {
    url = new URL(urlInput);
  } catch {
    return {
      valid: false,
      message: 'SCIM endpoint is invalid',
    };
  }

  if (url.protocol !== 'https:') {
    return {
      valid: false,
      message: 'SCIM endpoint must be an HTTPS endpoint',
    };
  }

  return {
    valid: true,
  };
};
