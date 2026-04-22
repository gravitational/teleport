/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

import { createContext, Dispatch, HTMLProps, SetStateAction } from 'react';

/**
 * Provides shared state for a single `DropdownMenu` level. Each menu level
 * creates its own provider so nested menus have independent state.
 */
export const DropdownMenuContext = createContext<{
  getItemProps: (userProps?: HTMLProps<HTMLElement>) => Record<string, unknown>;
  activeIndex: number | null;
  setActiveIndex: Dispatch<SetStateAction<number | null>>;
  isOpen: boolean;
  /** Close the menu, firing the `onOpenChange` callback. */
  closeMenu: () => void;
  search: string;
  setSearch: Dispatch<SetStateAction<string>>;
}>({
  getItemProps: () => ({}),
  activeIndex: null,
  setActiveIndex: () => {},
  isOpen: false,
  closeMenu: () => {},
  search: '',
  setSearch: () => {},
});
