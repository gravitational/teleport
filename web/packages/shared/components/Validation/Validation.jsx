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

import React from 'react';

import { isObject } from 'shared/utils/highbar';

import Logger from '../../libs/logger';

const logger = Logger.create('validation');

// Validator handles input validation
export default class Validator {
  valid = true;

  constructor() {
    // store subscribers
    this._subs = [];
  }

  // adds a callback to the list of subscribers
  subscribe(cb) {
    this._subs.push(cb);
  }

  // removes a callback from the list of subscribers
  unsubscribe(cb) {
    const index = this._subs.indexOf(cb);
    if (index > -1) {
      this._subs.splice(index, 1);
    }
  }

  addResult(result) {
    // result can be a boolean value or an object
    let isValid = false;
    if (isObject(result)) {
      isValid = result.valid;
    } else {
      logger.error(`rule should return a valid object`);
    }

    this.valid = this.valid && Boolean(isValid);
  }

  reset() {
    this.valid = true;
    this.validating = false;
  }

  validate() {
    this.reset();
    this.validating = true;
    this._subs.forEach(cb => {
      try {
        cb();
      } catch (err) {
        logger.error(err);
      }
    });

    return this.valid;
  }
}

const ValidationContext = React.createContext({});

export function Validation(props) {
  const [validator] = React.useState(() => new Validator());
  // handle render functions
  const children =
    typeof props.children === 'function'
      ? props.children({ validator })
      : props.children;

  return (
    <ValidationContext.Provider value={validator}>
      {children}
    </ValidationContext.Provider>
  );
}

export function useValidation() {
  const value = React.useContext(ValidationContext);
  if (!(value instanceof Validator)) {
    logger.warn('Missing Validation Context declaration');
  }

  return value;
}
