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

import React, { useState } from 'react';

import Validation, { Validator } from 'shared/components/Validation';

import { SectionProps } from './sections';

/** A helper for testing editor section components. */
export function StatefulSection<Spec, ValidationResult>({
  defaultValue,
  component: Component,
  onChange,
  validatorRef,
  validate,
}: {
  defaultValue: Spec;
  component: React.ComponentType<SectionProps<Spec, any>>;
  onChange(spec: Spec): void;
  validatorRef?(v: Validator): void;
  validate(arg: Spec): ValidationResult;
}) {
  const [model, setModel] = useState<Spec>(defaultValue);
  const validation = validate(model);
  return (
    <Validation>
      {({ validator }) => {
        validatorRef?.(validator);
        return (
          <Component
            value={model}
            validation={validation}
            isProcessing={false}
            onChange={spec => {
              setModel(spec);
              onChange(spec);
            }}
          />
        );
      }}
    </Validation>
  );
}
