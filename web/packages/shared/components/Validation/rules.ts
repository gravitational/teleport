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

type RuleResult<R extends Rule> = ReturnType<ReturnType<R>>;

/**
 * requiredField checks for empty strings and arrays.
 *
 * @param message The custom error message to display to users.
 * @param value The value user entered.
 */
const requiredField =
  <T = string>(message: string): Rule<T | T[] | readonly T[]> =>
  value =>
  () => {
    // TODO(bl-nero): This typecast hides the fact that `requiredField` doesn't
    // actually work for other primitive types, like `number`.
    const valid = !(!value || (value as T[]).length === 0);
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

/**
 * @param name validAwsIAMRoleName verifies if the given value is a
 * valid AWS IAM role name.
 */
const validAwsIAMRoleName = (name: string): ValidationResult => {
  if (name.length > 64) {
    return {
      valid: false,
      message: 'name should be <= 64 characters',
    };
  }

  if (!isIamRoleNameValid(name)) {
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
 * requiredIamRoleName is a required field and checks for a
 * value which should also be a valid AWS IAM role name.
 * @param name is a role name.
 * @returns ValidationResult
 */
const requiredIamRoleName: Rule = name => (): ValidationResult => {
  if (!name) {
    return {
      valid: false,
      message: 'IAM role name required',
    };
  }

  return validAwsIAMRoleName(name);
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
 * requiredMatchingRoleNameAndRoleArn checks if a given roleArn is a valid AWS
 * IAM role ARN format and contains a given roleName.
 *
 * @param roleName Role name that is used to match role ARN.
 * @param roleArn Role ARN which is to be tested for a valid AWS IAM role ARN format.
 */
const requiredMatchingRoleNameAndRoleArn =
  (roleName: string) => (roleArn: string) => () => {
    const regex = new RegExp(
      '^arn:aws.*:iam::\\d{12}:role\\/(' + roleName + ')$'
    );

    if (regex.test(roleArn)) {
      return {
        valid: true,
      };
    }

    return {
      valid: false,
      message:
        'invalid role ARN, double check you copied and pasted the correct output',
    };
  };

/**
 * requiredPort checks if the given value is a valid port value [1-65535].
 */
const requiredPort: Rule = port => () => {
  let val = Number(port);
  if (Number.isInteger(val) && val > 0 && val <= 65535) {
    return {
      valid: true,
    };
  }
  return {
    valid: false,
    message: 'Port required [1-65535]',
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

/** A result of the {@link arrayOf} validation rule. */
export type ArrayValidationResult<R = ValidationResult> = ValidationResult & {
  /** Results of validating each separate item. */
  results: R[];
};

/** Validates an array by executing given rule on each of its elements. */
const arrayOf =
  <T, R extends ValidationResult>(
    elementRule: Rule<T, R>
  ): Rule<T[], ArrayValidationResult<R>> =>
  (values: T[]) =>
  () => {
    const results = values.map(v => elementRule(v)());
    return { results: results, valid: results.every(r => r.valid) };
  };

/**
 * Passes a precomputed validation result instead of computing it inside the
 * rule.
 *
 * This rule is a hacky way to allow the validation engine to operate with
 * validation results computed outside of the validator's validation cycle. See
 * the `Validation` component's documentation for more information about where
 * this is useful and a detailed usage example.
 */
const precomputed =
  <T>(res: ValidationResult): Rule<T> =>
  () =>
  () =>
    res;

/**
 * A set of rules to be executed using `runRules` on a model object. The rule
 * set contains a subset of keys of the object.
 */
export type RuleSet<K extends string | number | symbol> = Record<
  K,
  Rule<any, any>
>;

/** A result of executing a set of rules on a model object. */
export type RuleSetValidationResult<R extends RuleSet<any>> = {
  valid: boolean;
  /**
   * Each member of the `fields` object corresponds to a rule from within the
   * rule set and contains the result of validating a model field of the same
   * name.
   */
  fields: { [k in keyof R]: RuleResult<R[k]> }; // Record<keyof R, ValidationResult>;
};

/**
 * Executes a set of rules on a model object, producing a precomputed
 * validation result that can be used with `precomputed` rule to inject to
 * field components, but also allows for consuming the validation data outside
 * these fields.
 *
 * `K` is the subset of model field names.
 * `M` is the validated model.
 */
export const runRules = <K extends string, M extends Record<K, any>>(
  model: M,
  rules: RuleSet<K>
): RuleSetValidationResult<RuleSet<K>> => {
  const fields = {} as {
    [k in keyof RuleSet<K>]: RuleResult<RuleSet<K>[k]>;
  };
  let valid = true;
  for (const key in rules) {
    const modelValue = model[key];
    fields[key] = rules[key](modelValue)();
    valid &&= fields[key].valid;
  }
  return { fields, valid };
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
  requiredMatchingRoleNameAndRoleArn,
  validAwsIAMRoleName,
  requiredPort,
  arrayOf,
  precomputed,
};
