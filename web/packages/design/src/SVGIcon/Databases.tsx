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

import React from 'react';

import { SVGIcon } from './SVGIcon';

import type { SVGIconProps } from './common';

export function DatabasesIcon({ size = 20, fill }: SVGIconProps) {
  return (
    <SVGIcon viewBox="0 0 19 20" size={size} fill={fill}>
      <path d="M9.41675 1.25C13.5574 1.25 16.9167 2.37305 16.9167 3.75V5C16.9167 6.37695 13.5574 7.5 9.41675 7.5C5.27612 7.5 1.91675 6.37695 1.91675 5V3.75C1.91675 2.37305 5.27612 1.25 9.41675 1.25ZM16.9167 7.08984V8.75C16.9167 10.127 13.5574 11.25 9.41675 11.25C5.27612 11.25 1.91675 10.127 1.91675 8.75V7.08984C3.52808 8.22266 6.47729 8.75 9.41675 8.75C12.3562 8.75 15.3054 8.22266 16.9167 7.08984ZM16.9167 10.8398V12.5C16.9167 13.877 13.5574 15 9.41675 15C5.27612 15 1.91675 13.877 1.91675 12.5V10.8398C3.52808 11.9727 6.47729 12.5 9.41675 12.5C12.3562 12.5 15.3054 11.9727 16.9167 10.8398ZM16.9167 14.5898V16.25C16.9167 17.627 13.5574 18.75 9.41675 18.75C5.27612 18.75 1.91675 17.627 1.91675 16.25V14.5898C3.52808 15.7227 6.47729 16.25 9.41675 16.25C12.3562 16.25 15.3054 15.7227 16.9167 14.5898ZM9.41675 0C6.36433 0 0.666748 0.734414 0.666748 3.75V16.25C0.666748 19.271 6.37362 20 9.41675 20C12.4692 20 18.1667 19.2656 18.1667 16.25V3.75C18.1667 0.728984 12.4599 0 9.41675 0Z" />
    </SVGIcon>
  );
}
