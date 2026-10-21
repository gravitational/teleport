/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import { SearchInput } from '@gravitational/design-system';
import { type ChangeEvent, Ref, useMemo, useRef, useState } from 'react';

import { mergeRefs } from 'design/utils/mergeRefs';
import { debounce } from 'shared/utils/highbar';

/**
 * A search input dedicated for use with lists of scopes, with a built-in
 * change debounce.
 */
export function ScopeSearchInput({
  onChange,
  ref,
  noIcon,
  autoFocus,
}: {
  /** A debounced search input change handler. */
  onChange: (value: string) => void;
  ref?: Ref<HTMLInputElement>;
  /** If set to true, the input doesn't display the magnifying glass icon. */
  noIcon?: boolean;
  /** Determines if the input focuses automatically when mounted. */
  autoFocus?: boolean;
}) {
  const inputRef = useRef<HTMLInputElement | null>(null);
  const [inputValue, setInputValue] = useState('');
  const mergedInputRef = useMemo(() => mergeRefs([inputRef, ref]), [ref]);

  const debouncedOnChange = useMemo(() => {
    return debounce((prefix: string) => {
      onChange(prefix);
    }, 500);
  }, [onChange]);

  function onInputValueChanged(e: ChangeEvent<HTMLInputElement>) {
    setInputValue(e.target.value);
    debouncedOnChange(e.target.value);
  }

  const iconProp = noIcon ? { icon: undefined } : {};

  return (
    <SearchInput
      placeholder="Search for scopes..."
      ref={mergedInputRef}
      value={inputValue}
      onChange={onInputValueChanged}
      autoFocus={autoFocus}
      {...iconProp}
    />
  );
}
