/**
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

import { ComponentPropsWithRef, ComponentType, CSSProperties } from 'react';

import { Flex } from 'design';
import { ButtonBorder } from 'design/Button';

export type LoginItem = {
  url: string;
  login: string;
  /** Indicates url to be launched with an href attribute and in a new tab. */
  isExternalUrl?: boolean;
};

// MenuInputType determines how the input present in the MenuLogin
// will function. Default is Input, which allows freeform input and
// will call the `onSelect` function with whatever value is entered.
// FILTER will only filter the options present in the list and will
// pass the 0th item in the filtered list to `onSelect` instead.
export enum MenuInputType {
  INPUT,
  FILTER,
  NONE,
}

export type MenuLoginProps = {
  getLoginItems: () => LoginItem[] | Promise<LoginItem[]>;
  /**
   * If isExternalUrl of login item is true, a button with <a> tag is rendered
   * and the value of url is passed for the login param. Since <a> tag with href
   * attribute handles onClick by default, the caller may wish to
   * pass an empty onSelect function value.
   */
  onSelect: (e: React.SyntheticEvent, login: string) => void;
  anchorOrigin?: any;
  inputType?: MenuInputType;
  alignButtonWidthToMenu?: boolean;
  transformOrigin?: any;
  textTransform?: string;
  placeholder?: string;
  required?: boolean;
  /**
   * Width of the displayed menu. If alignButtonWidthToMenu is true, then this is also the width of
   * the button.
   */
  width?: ComponentPropsWithRef<typeof Flex>['width'];
  /**
   * Width of just the button. Ignored if alignButtonWidthToMenu is true.
   */
  buttonWidth?: ComponentPropsWithRef<typeof ButtonBorder>['width'];
  ButtonComponent?: ComponentType<ComponentPropsWithRef<typeof ButtonBorder>>;
  buttonText?: string;
  style?: CSSProperties;
};

export type MenuLoginHandle = {
  open: () => void;
};
