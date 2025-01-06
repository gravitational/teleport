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

import { Logger } from 'design/logger';
import { Store, useStore } from 'shared/libs/stores';
import { isObject } from 'shared/utils/highbar';

import { ValidationResult } from './rules';

const logger = new Logger('validation');

/** A per-rule callback that will be executed during validation. */
type RuleCallback = () => void;

export type Result = ValidationResult | boolean;

type ValidatorState = {
  /** Indicates whether the last validation was successful. */
  valid: boolean;
  /**
   * Indicates whether the validator has been activated by a call to
   * `validate`.
   */
  validating: boolean;
};

/** A store that handles input validation and makes its results accessible. */
export default class Validator extends Store<ValidatorState> {
  state = {
    valid: true,
    validating: false,
  };

  /** Callbacks that will be executed upon validation. */
  private ruleCallbacks: RuleCallback[] = [];

  /** Adds a rule callback that will be executed upon validation. */
  addRuleCallback(cb: RuleCallback) {
    this.ruleCallbacks.push(cb);
  }

  /** Removes a rule callback. */
  removeRuleCallback(cb: RuleCallback) {
    const index = this.ruleCallbacks.indexOf(cb);
    if (index > -1) {
      this.ruleCallbacks.splice(index, 1);
    }
  }

  addResult(result: Result) {
    // result can be a boolean value or an object
    let isValid = false;
    if (isObject(result)) {
      isValid = result.valid;
    } else {
      logger.error(`rule should return a valid object`);
    }

    this.setState({ valid: this.state.valid && Boolean(isValid) });
  }

  reset() {
    this.setState({
      valid: true,
      validating: false,
    });
  }

  validate() {
    this.reset();
    this.setState({ validating: true });
    for (const cb of this.ruleCallbacks) {
      try {
        cb();
      } catch (err) {
        logger.error(err);
      }
    }

    return this.state.valid;
  }
}

const ValidationContext = React.createContext<Validator | undefined>(undefined);

type ValidationRenderFunction = (arg: {
  validator: Validator;
}) => React.ReactNode;

/**
 * Installs a validation context that provides a {@link Validator} store. The
 * store can be retrieved either through {@link useValidation} hook or by a
 * render callback, e.g.:
 *
 * ```
 * function Component() {
 *   return (
 *     <Validation>
 *       {({validator}) => (
 *         <>
 *           (...)
 *           <button onClick={() => validator.validate()}>Validate</button>
 *         </>
 *       )}
 *     </Validation>
 *   );
 * }
 * ```
 *
 * The simplest way to use validation is validating on the view layer: just use
 * a `rule` prop with `FieldInput` or a similar component and pass a rule like
 * `requiredField`.
 *
 * Unfortunately, due to architectural limitations, this will not work well in
 * scenarios where information about validity about given field or group of
 * fields is required outside that field. In cases like this, the best option
 * is to validate the model during render time on the top level (for example,
 * execute an entire set of rules on a model using `runRules`). The result of
 * model validation will then contain information about the validity of each
 * field. It can then be used wherever it's needed, and also attached to
 * appropriate inputs with a `precomputed` validation rule. Example:
 *
 * ```
 * function Component(model: Model) {
 *   const rules = {
 *     name: requiredField('required'),
 *     email: requiredEmailLike,
 *   }
 *   const validationResult = runRules(model, rules);
 * }
 * ```
 *
 * Note that, as this example shows clearly, the validator itself, despite its
 * name, doesn't really validate anything -- it merely aggregates validation
 * results. Also it's worth mentioning that the validator will not do it
 * without our help -- each validated field needs to be actually attached to a
 * field, even if using a `precomputed` rule, for this to work. The validation
 * callbacks registered by validation rules on the particular fields are the
 * actual points where the errors are consumed by the validator.
 */
export function Validation(props: {
  children?: React.ReactNode | ValidationRenderFunction;
}) {
  const [validator] = React.useState(() => new Validator());
  useStore(validator);
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

export function useValidation(): Validator {
  const validator = React.useContext(ValidationContext);
  if (!validator) {
    throw new Error('useValidation() called without a validation context');
  }
  return useStore(validator);
}
