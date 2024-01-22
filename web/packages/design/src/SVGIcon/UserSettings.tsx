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

export function UserSettingsIcon({ size = 18, fill }: SVGIconProps) {
  return (
    <SVGIcon viewBox="0 0 18 18" size={size} fill={fill}>
      <path d="M16.5879 2.94223L9.83789 0.129375C9.63047 0.0417656 9.40898 0 9.19102 0C8.97305 0 8.74805 0.0417656 8.54063 0.129656L1.79062 2.94251C1.16203 3.20168 0.75 3.81797 0.75 4.46836C0.75 13.5492 7.40859 18 9.15586 18C10.9172 18 17.625 13.616 17.625 4.46836C17.625 3.81797 17.2137 3.20168 16.5879 2.94223ZM16.4965 4.51055C16.4965 12.4313 10.6535 16.875 9.19102 16.875C7.68633 16.8434 1.875 12.382 1.875 4.5C1.875 4.27148 2.01123 4.06934 2.21777 3.98145L8.96777 1.16895C9.03714 1.13985 9.11223 1.12444 9.18778 1.12444C9.26059 1.12444 9.33382 1.13875 9.40283 1.16895L16.1528 3.98145C16.4754 4.1168 16.4965 4.42266 16.4965 4.51055ZM9.1875 5.34375C8.10188 5.34375 7.21875 6.22688 7.21875 7.3125C7.21875 8.20125 7.81465 8.94551 8.625 9.18949V11.8125C8.625 12.1234 8.87658 12.375 9.1875 12.375C9.49842 12.375 9.75 12.1234 9.75 11.8125V9.18984C10.5586 8.94727 11.1562 8.20195 11.1562 7.3125C11.1562 6.22617 10.2738 5.34375 9.1875 5.34375ZM9.1875 8.15625C8.72273 8.15625 8.34375 7.77727 8.34375 7.3125C8.34375 6.84773 8.72344 6.46875 9.1875 6.46875C9.65156 6.46875 10.0312 6.84773 10.0312 7.3125C10.0312 7.77727 9.65156 8.15625 9.1875 8.15625Z" />
    </SVGIcon>
  );
}
