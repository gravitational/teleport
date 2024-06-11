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

import { IAM_ROLE_NAME_REGEX } from 'teleport/services/integrations/aws';

/**
 * The result of validating a field.
 */
export interface ValidationResult {
  valid: boolean;
  message?: string;
}

/**
 * A function to validate a field value.
 */
export type Rule<T = string, R = ValidationResult> = (value: T) => () => R;

/**
 * requiredField checks for empty strings and arrays.
 *
 * @param message The custom error message to display to users.
 * @param value The value user entered.
 */
const requiredField =
  <T = string>(message: string): Rule<string | T[]> =>
  value =>
  () => {
    const valid = !(!value || value.length === 0);
    return {
      valid,
      message: !valid ? message : '',
    };
  };

const requiredToken: Rule = value => () => {
  if (!value || value.length === 0) {
    return {
      valid: false,
      message: 'Token is required',
    };
  }

  return {
    valid: true,
  };
};

const requiredPassword: Rule = value => () => {
  if (!value || value.length < 12) {
    return {
      valid: false,
      message: 'Enter at least 12 characters',
    };
  }

  return {
    valid: true,
  };
};

const requiredConfirmedPassword =
  (password: string): Rule =>
  (confirmedPassword: string) =>
  () => {
    if (!confirmedPassword) {
      return {
        valid: false,
        message: 'Please confirm your password',
      };
    }

    if (confirmedPassword !== password) {
      return {
        valid: false,
        message: 'Password does not match',
      };
    }

    return {
      valid: true,
    };
  };

const isIamRoleNameValid = roleName => {
  return (
    roleName && roleName.length <= 64 && roleName.match(IAM_ROLE_NAME_REGEX)
  );
};

const requiredIamRoleName: Rule = value => () => {
  if (!value) {
    return {
      valid: false,
      message: 'IAM role name required',
    };
  }

  if (value.length > 64) {
    return {
      valid: false,
      message: 'name should be <= 64 characters',
    };
  }

  if (!isIamRoleNameValid(value)) {
    return {
      valid: false,
      message: 'name can only contain characters @ = , . + - and alphanumerics',
    };
  }

  return {
    valid: true,
  };
};

/**
 * ROLE_ARN_REGEX_STR checks provided arn (amazon resource names) is
 * somewhat in the format as documented here:
 * https://docs.aws.amazon.com/IAM/latest/UserGuide/reference-arns.html
 *
 * The regex is in string format, and must be parsed with `new RegExp()`.
 *
 * regex details:
 * arn:aws<OTHER_PARTITION>:iam::<ACOUNT_NUMBER>:role/<ROLE_NAME>
 */
const ROLE_ARN_REGEX_STR = '^arn:aws.*:iam::\\d{12}:role\\/';
const requiredRoleArn: Rule = roleArn => () => {
  if (!roleArn) {
    return {
      valid: false,
      message: 'role ARN required',
    };
  }

  const regex = new RegExp(ROLE_ARN_REGEX_STR + '(.*)$');
  const match = roleArn.match(regex);

  if (!match || !match[1] || !isIamRoleNameValid(match[1])) {
    return {
      valid: false,
      message: 'invalid role ARN format',
    };
  }

  return {
    valid: true,
  };
};

export interface EmailValidationResult extends ValidationResult {
  kind?: 'empty' | 'invalid';
}

// requiredEmailLike ensures a string contains a plausible email, i.e. that it
// contains an '@' and some characters on each side.
const requiredEmailLike: Rule<string, EmailValidationResult> = email => () => {
  if (!email) {
    return {
      valid: false,
      kind: 'empty',
      message: 'Email address is required',
    };
  }

  // Must contain an @, i.e. 2 entries, and each must be nonempty.
  let parts = email.split('@');
  if (parts.length !== 2 || !parts[0] || !parts[1]) {
    return {
      valid: false,
      kind: 'invalid',
      message: `Email address '${email}' is invalid`,
    };
  }

  return {
    valid: true,
  };
};

/**
 * A rule function that combines multiple inner rule functions. All rules must
 * return `valid`, otherwise it returns a comma separated string containing all
 * invalid rule messages.
 * @param rules a list of rule functions to apply
 * @returns a rule function that ANDs all input rules
 */
const requiredAll =
  <T>(...rules: Rule<T | string | string[], ValidationResult>[]): Rule<T> =>
  (value: T) =>
  () => {
    let messages = [];
    for (let r of rules) {
      let result = r(value)();
      if (!result.valid) {
        messages.push(result.message);
      }
    }

    if (messages.length > 0) {
      return {
        valid: false,
        message: messages.join('. '),
      };
    }
    return { valid: true };
  };

export {
  requiredToken,
  requiredPassword,
  requiredConfirmedPassword,
  requiredField,
  requiredRoleArn,
  requiredIamRoleName,
  requiredEmailLike,
  requiredAll,
};
