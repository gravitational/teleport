/**
 * Copyright 2020 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';

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

  const [, rerender] = React.useState();
  const validator = useValidation();

  // register to validation context to be called on cb()
  React.useEffect(() => {
    function onValidate() {
      if (validator.validating) {
        const result = cb();
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
  }, [cb]);

  // if validation has been requested, cb right away.
  if (validator.validating) {
    return cb();
  }

  return { valid: true };
}
