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

export function DownloadsIcon({ size = 18, fill }: SVGIconProps) {
  return (
    <SVGIcon viewBox="0 0 18 18" size={size} fill={fill}>
      <path d="M8.62734 13.3277C8.73281 13.4543 8.86641 13.5 9 13.5C9.13359 13.5 9.26698 13.4528 9.37336 13.3577L14.4359 8.85772C14.6677 8.65174 14.6897 8.29593 14.4831 8.06389C14.2766 7.83098 13.9185 7.80957 13.6889 8.01721L9.5625 11.6859V0.5625C9.5625 0.251578 9.30937 0 9 0C8.69063 0 8.4375 0.251578 8.4375 0.5625V11.6859L4.31016 8.01562C4.08164 7.8082 3.72305 7.83281 3.51562 8.06484C3.31031 8.26523 3.33211 8.65195 3.56484 8.82773L8.62734 13.3277ZM15.75 11.25H14.0625C13.7519 11.25 13.5 11.5018 13.5 11.8125C13.5 12.1231 13.7519 12.375 14.0625 12.375H15.75C16.3712 12.375 16.875 12.8788 16.875 13.5V15.75C16.875 16.3712 16.3712 16.875 15.75 16.875H2.25C1.62879 16.875 1.125 16.3712 1.125 15.75V13.5C1.125 12.8788 1.62879 12.375 2.25 12.375H3.9375C4.24687 12.375 4.5 12.1219 4.5 11.8125C4.5 11.5031 4.24687 11.25 3.9375 11.25H2.25C1.00723 11.25 0 12.2572 0 13.5V15.75C0 16.9928 1.00723 18 2.25 18H15.75C16.9928 18 18 16.9928 18 15.75V13.5C18 12.259 16.991 11.25 15.75 11.25ZM15.4688 14.625C15.4688 14.1592 15.0908 13.7812 14.625 13.7812C14.1592 13.7812 13.7812 14.1592 13.7812 14.625C13.7812 15.0908 14.1592 15.4688 14.625 15.4688C15.0908 15.4688 15.4688 15.0926 15.4688 14.625Z" />
    </SVGIcon>
  );
}
