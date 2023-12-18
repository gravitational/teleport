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

export function ConversationListIcon({ size = 24, fill }: SVGIconProps) {
  return (
    <SVGIcon fill={fill} size={size} viewBox="0 0 24 24">
      <path d="M18 8.016V6H6v2.016h12zm-3.984 6V12H6v2.016h8.016zM6 9v2.016h12V9H6zm14.016-6.984q.797 0 1.383.586t.586 1.383v12q0 .797-.586 1.406T20.016 18H6l-3.984 3.984v-18q0-.797.586-1.383t1.383-.586h16.031z" />
    </SVGIcon>
  );
}
