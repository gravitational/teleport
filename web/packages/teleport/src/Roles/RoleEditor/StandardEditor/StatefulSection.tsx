/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import React, { useReducer } from 'react';

import Validation, { Validator } from 'shared/components/Validation';

import { SectionProps, SectionPropsWithDispatch } from './sections';
import { defaultRoleVersion, StandardEditorModel } from './standardmodel';
import { useStandardModel } from './useStandardModel';
import { withDefaults } from './withDefaults';

/** A helper for testing editor section components. */
export function StatefulSection<Model, ValidationResult>({
  defaultValue,
  component: Component,
  onChange,
  validatorRef,
  validate,
}: {
  defaultValue: Model;
  component: React.ComponentType<SectionProps<Model, any>>;
  onChange(model: Model): void;
  validatorRef?(v: Validator): void;
  validate(
    model: Model,
    previousModel: Model,
    previousResult: ValidationResult
  ): ValidationResult;
}) {
  const [{ model, validationResult }, dispatch] = useReducer(
    (
      { model, validationResult: previousValidationResult },
      newModel: Model
    ) => ({
      model: newModel,
      validationResult: validate(newModel, model, previousValidationResult),
    }),
    {
      model: defaultValue,
      validationResult: validate(defaultValue, undefined, undefined),
    }
  );
  return (
    <Validation>
      {({ validator }) => {
        validatorRef?.(validator);
        return (
          <Component
            value={model}
            validation={validationResult}
            isProcessing={false}
            onChange={newModel => {
              dispatch(newModel);
              onChange(newModel);
            }}
          />
        );
      }}
    </Validation>
  );
}

const minimalRole = withDefaults({
  metadata: { name: 'foobar' },
  version: defaultRoleVersion,
});

/** A helper for testing editor section components. */
export function StatefulSectionWithDispatch<Model, ValidationResult>({
  selector,
  validationSelector,
  component: Component,
  validatorRef,
  modelRef,
}: {
  selector(m: StandardEditorModel): Model;
  validationSelector(m: StandardEditorModel): ValidationResult;
  component: React.ComponentType<SectionPropsWithDispatch<Model, any>>;
  validatorRef?(v: Validator): void;
  modelRef?(m: Model): void;
}) {
  const [state, dispatch] = useStandardModel(minimalRole);
  const model = selector(state);
  const validation = validationSelector(state);
  modelRef?.(model);
  return (
    <Validation>
      {({ validator }) => {
        validatorRef?.(validator);
        return (
          <Component
            value={model}
            validation={validation}
            isProcessing={false}
            dispatch={dispatch}
          />
        );
      }}
    </Validation>
  );
}
