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

import { useEffect, useState } from 'react';

import Logger from '../../libs/logger';
import { useValidation } from './Validation';

const logger = Logger.create('validation');

/**
 * useRule subscribes to validation requests upon which executes validate() callback
 */
export default function useRule(cb) {
  if (typeof cb !== 'function') {
    logger.warn(`useRule(fn), fn() must be a function`);
    return;
  }

  const [, rerender] = useState();
  const validator = useValidation();

  // register to validation context to be called on cb()
  useEffect(() => {
    function onValidate() {
      if (validator.state.validating) {
        const result = cb();
        validator.addResult(result);
        rerender({});
      }
    }

    // subscribe to store changes
    validator.addRuleCallback(onValidate);

    // unsubscribe on unmount
    function cleanup() {
      validator.removeRuleCallback(onValidate);
    }

    return cleanup;
  }, [cb]);

  // if validation has been requested, cb right away.
  if (validator.state.validating) {
    return cb();
  }

  return { valid: true };
}
