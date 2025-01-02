/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import FieldInput from 'shared/components/FieldInput';
import { Option } from 'shared/components/Select';
import { requiredField } from 'shared/components/Validation/rules';
import { capitalizeFirstLetter } from 'shared/utils/text';

import { WILD_CARD } from './const';

export function CustomInputFieldForAsterisks({
  selectedOption,
  value,
  onValueChange,
  disabled,
  nameKind,
}: {
  selectedOption: Option;
  value: string;
  onValueChange(s: string): void;
  disabled: boolean;
  nameKind: string;
}) {
  if (!selectedOption || selectedOption?.value !== WILD_CARD) {
    return null;
  }

  return (
    <FieldInput
      mt={2}
      mb={0}
      autoFocus={true}
      label={`Enter a custom ${nameKind} name:`}
      value={value}
      onChange={e => onValueChange(e.target.value)}
      disabled={disabled}
      placeholder={`custom-${nameKind.replace(' ', '-')}-name`}
      rule={requiredField(
        `${capitalizeFirstLetter(nameKind)} name is required`
      )}
    />
  );
}
