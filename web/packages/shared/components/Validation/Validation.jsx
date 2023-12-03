/*
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
