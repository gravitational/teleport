/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { Option } from 'shared/components/Select';

/**
 * Represents an error that occurred during converting a field of the role
 * resource to its editor model. These errors are not fatal, and as such, are
 * collected and returned along with the converted model.
 */
export type ConversionError =
  | SimpleConversionError
  | ConversionErrorWithReplacement;

export enum ConversionErrorType {
  /** Field has an unsupported value and will be removed. */
  UnsupportedValue = 'value',
  /** Field is unsupported and will be removed. */
  UnsupportedField = 'field',
  /** Field has an unsupported value and will be replaced. */
  UnsupportedValueWithReplacement = 'value-with-replacement',
  /** Changing this field is not supported. */
  UnsupportedChange = 'change',
}

export type SimpleConversionErrorType =
  | ConversionErrorType.UnsupportedValue
  | ConversionErrorType.UnsupportedField
  | ConversionErrorType.UnsupportedChange;

type ConversionErrorBase<T extends ConversionErrorType> = {
  type: T;
  /**
   * A full path of the field in the role resource that this error is about.
   */
  path: string;
};

/** An error that doesn't have any additional fields. */
export type SimpleConversionError =
  ConversionErrorBase<SimpleConversionErrorType>;

/**
 * An error that carries information about a replacement value for the field.
 */
export type ConversionErrorWithReplacement =
  ConversionErrorBase<ConversionErrorType.UnsupportedValueWithReplacement> & {
    /**
     * A serialized representation of a value that this field will be replaced
     * with.
     */
    replacement: string;
  };

/** A group of conversion errors of a given type. */
export type ConversionErrorGroup = {
  type: ConversionErrorType;
  errors: ConversionError[];
};

/**
 * A utility function that creates conversion errors for a group of unsupported
 * fields.
 * @param prefix Field path of the object that contained these unsupported
 *   fields.
 * @param obj An object that contains unsupported fields. Easy to obtain by
 *   destructuring the known fields and collecting the rest.
 */
export function unsupportedFieldErrorsFromObject(
  prefix: string,
  obj: Record<string, any>
): ConversionError[] {
  const keys = Object.keys(obj);
  return simpleConversionErrors(
    ConversionErrorType.UnsupportedField,
    prefix ? keys.map(key => `${prefix}.${key}`) : keys
  );
}

/**
 * A utility function that creates a list of conversion errors of a given type
 * for a given list of paths.
 */
export function simpleConversionErrors<T extends SimpleConversionErrorType>(
  type: T,
  paths: string[]
): SimpleConversionError[] {
  return paths.map(path => ({ type, path }));
}

/**
 * Creates a {@link ConversionErrorType.UnsupportedValueWithReplacement} error.
 */
export function unsupportedValueWithReplacement(
  path: string,
  replacement: any
): ConversionErrorWithReplacement {
  return {
    type: ConversionErrorType.UnsupportedValueWithReplacement,
    path,
    replacement: JSON.stringify(replacement),
  };
}

/**
 * Retrieves an {@link Option} that corresponds to a given value. If the value
 * is not allowed (i.e. it's not contained in the options map), pushes an error
 * to the given errors list and returns a default option.
 */
export function getOptionOrPushError<V>(
  value: V,
  optionsMap: Map<V, Option<V>>,
  defaultValue: V,
  fieldPath: string,
  errors: ConversionError[]
) {
  const opt = optionsMap.get(value);
  if (opt !== undefined) {
    return opt;
  }
  errors.push(unsupportedValueWithReplacement(fieldPath, defaultValue));
  return optionsMap.get(defaultValue);
}

type GroupedErrors = Partial<Record<ConversionErrorType, ConversionError[]>>;

function groupErrors(errors: ConversionError[]): GroupedErrors {
  // We can't use Object.groupBy just yet because of Node.js version.
  const grouped: GroupedErrors = {};
  for (const e of errors) {
    if (!(e.type in grouped)) {
      grouped[e.type] = [];
    }
    grouped[e.type].push(e);
  }
  return grouped;
}

/**
 * Returns conversion errors grouped by type and sorted by path within each
 * group. The returned value is a list, not an object or map, to make sure that
 * we have full control over the order, which will be reflected on the UI.
 * However, it's guaranteed that each error type has no more than one group.
 */
export function groupAndSortConversionErrors(
  errors: ConversionError[]
): ConversionErrorGroup[] {
  const grouped = groupErrors(errors);
  const result: ConversionErrorGroup[] = [];

  // Add groups for known conversion error types in desired order.
  for (const type of [
    ConversionErrorType.UnsupportedField,
    ConversionErrorType.UnsupportedValue,
    ConversionErrorType.UnsupportedValueWithReplacement,
    ConversionErrorType.UnsupportedChange,
  ]) {
    const group = grouped[type];
    if (group) {
      result.push({ type, errors: group });
      delete grouped[type];
    }
  }

  // If someone adds an error type, but forgets to mention it above, we can
  // still add it to the groups; the order may me not guaranteed and unstable,
  // but it's better than not reporting it at all.
  for (const t in grouped) {
    result.push({ type: t as ConversionErrorType, errors: grouped[t] });
  }

  // Sort the groups by path.
  for (const group of result) {
    group.errors.sort((a, b) => a.path.localeCompare(b.path));
  }

  return result;
}
