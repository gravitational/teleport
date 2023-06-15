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

export type Props = {
  inputId?: string;
  hasError?: boolean;
  isClearable?: boolean;
  isSimpleValue?: boolean;
  isSearchable?: boolean;
  isDisabled?: boolean;
  menuIsOpen?: boolean;
  hideSelectedOptions?: boolean;
  controlShouldRenderValue?: boolean;
  maxMenuHeight?: number;
  onChange(e: Option<any, any> | Option<any, any>[]): void;
  onKeyDown?(e: KeyboardEvent): void;
  value: null | Option<any, any> | Option<any, any>[];
  isMulti?: boolean;
  autoFocus?: boolean;
  label?: string;
  placeholder?: string;
  options: Option<any, any>[];
  width?: string | number;
  menuPlacement?: string;
  name?: string;
  minMenuHeight?: number;
  components?: any;
  customProps?: Record<string, any>;
  menuPosition?: 'fixed' | 'absolute';
  inputValue?: string;
  filterOption?(): null | boolean;
  onInputChange?(value: string, actionMeta: ActionMeta): void;
  // Whether or not the element is on an elevated platform (such as a dialog).
  elevated?: boolean;
};

export type AsyncProps = Omit<Props, 'options'> & {
  defaultOptions?: true | Option;
  cacheOptions?: boolean;
  defaultMenuIsOpen?: boolean;
  loadOptions(input: string, o?: Option[]): Promise<Option[] | void>;
  noOptionsMessage(): string;
};

// Option defines the data type for select dropdown list.
export type Option<T = string, S = string> = {
  // value is the actual value used inlieu of label.
  value: T;
  // label is the value user sees in the select options dropdown.
  label: S;
};

export type ActionMeta = {
  action: 'set-value' | 'input-change' | 'input-blur' | 'menu-close';
};
