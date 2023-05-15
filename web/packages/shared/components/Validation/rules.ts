/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

/**
 * requiredField checks for empty strings and arrays.
 *
 * @param message The custom error message to display to users.
 * @param value The value user entered.
 */
const requiredField = (message: string) => (value: string) => () => {
  const valid = !(!value || value.length === 0);
  return {
    valid,
    message: !valid ? message : '',
  };
};

const requiredToken = (value: string) => () => {
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

const requiredPassword = (value: string) => () => {
  if (!value || value.length < 6) {
    return {
      valid: false,
      message: 'Enter at least 6 characters',
    };
  }

  return {
    valid: true,
  };
};

const requiredConfirmedPassword =
  (password: string) => (confirmedPassword: string) => () => {
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

// requiredRoleArn checks provided arn (AWS role name) is somewhat
// in the format as documented here:
// https://docs.aws.amazon.com/IAM/latest/UserGuide/reference-arns.html
const requiredRoleArn = (roleArn: string) => () => {
  let parts = [];
  if (roleArn) {
    parts = roleArn.split(':role');
  }

  if (
    parts.length == 2 &&
    parts[0].startsWith('arn:aws:iam:') &&
    // the `:role` part can be followed by a forward slash or a colon,
    // followed by the role name.
    parts[1].length >= 2
  ) {
    return {
      valid: true,
    };
  }

  return {
    valid: false,
    message: 'invalid role ARN format',
  };
};

export {
  requiredToken,
  requiredPassword,
  requiredConfirmedPassword,
  requiredField,
  requiredRoleArn,
};
