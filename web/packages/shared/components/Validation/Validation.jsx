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
import Logger from '../../libs/logger';
import { isObject } from 'lodash';

const logger = Logger.create('validation');

// Validator handles input validation
export default class Validator {
  valid = true;

  values = {};

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
      if (result.name) {
        this.results.values[name] = result;
      }
    } else {
      logger.error(`rule should return a valid object`);
    }

    this.valid = this.valid && Boolean(isValid);
  }

  reset() {
    this.valid = true;
    this.values = {};
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
    typeof props.children === 'function' ? props.children({ validator }) : props.children;

  return <ValidationContext.Provider value={validator}>{children}</ValidationContext.Provider>;
}

export function useValidation() {
  const value = React.useContext(ValidationContext);
  if (!(value instanceof Validator)) {
    logger.warn('Missing Validation Context declaration');
  }

  return value;
}

/**
 * useRule registeres with validation context and runs validation function
 * after validation has been requested
 */
export function useRule(validate) {
  if (typeof validate !== 'function') {
    logger.warn(`useRule(fn), fn() must be a function`);
    return;
  }

  const [, rerender] = React.useState();
  const validator = useValidation();

  // register to validation context to be called on validate()
  React.useEffect(() => {
    function onValidate() {
      if (validator.validating) {
        const result = validate();
        validator.addResult(result);
        rerender({});
      }
    }

    // subscribe to store changes
    validator.subscribe(onValidate);

    // unsubscribe on unmount
    function cleanup() {
      validator.unsubscribe(onValidate);
    }

    return cleanup;
  }, [validate]);

  // if validation has been requested, validate right away.
  if (validator.validating) {
    return validate();
  }

  return { valid: true };
}
