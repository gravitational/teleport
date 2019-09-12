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

const requiredField = message => value => () => {
  return {
    valid: !!value,
    message: !value ? message : ''
  }
}

const requiredToken = value => () => {
  if (value.length === 0) {
    return {
      valid: false,
      message: 'Token is required',
    }
  }

  return {
    valid: true
  }
}

const requiredPassword = value => () => {
  if (value.length < 6) {
    return {
      valid: false,
      message: 'Enter at least 6 characters',
    }
  }

  return {
    valid: true
  }
}

const requiredConfirmedPassword = password => confirmedPassword => () => {
  if (!confirmedPassword) {
    return {
      valid: false,
      message: 'Please confirm your password',
    }
  }

  if (confirmedPassword !== password) {
    return {
      valid: false,
      message: 'Password does not match',
    }
  }

  return {
    valid: true
  }
}

export {
  requiredToken,
  requiredPassword,
  requiredConfirmedPassword,
  requiredField
}