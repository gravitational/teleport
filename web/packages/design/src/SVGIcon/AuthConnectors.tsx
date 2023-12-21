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

export function AuthConnectorsIcon({ size = 13, fill }: SVGIconProps) {
  return (
    <SVGIcon size={size} fill={fill} viewBox="0 0 14 14">
      <path
        fillRule="evenodd"
        clipRule="evenodd"
        d="M6.53931 13.9815C6.57519 13.9938 6.61281 13.9999 6.65 13.9999H6.65087C6.68806 13.9999 6.72569 13.9934 6.76156 13.9815C8.04869 13.5524 9.66437 12.1252 10.8776 10.3464C11.9827 8.72454 13.3009 6.03873 13.3009 2.44904C13.3009 2.25567 13.1442 2.09904 12.9509 2.09904C10.9808 2.09904 7.88419 0.750666 6.84469 0.0576663C6.727 -0.0206462 6.57387 -0.0206462 6.45619 0.0576663C5.41669 0.750666 2.31962 2.09904 0.35 2.09904C0.156625 2.09904 0 2.25567 0 2.44904C0 6.03873 1.31819 8.72498 2.42331 10.3464C3.6365 12.1257 5.25219 13.5524 6.53931 13.9815ZM3.00169 9.95304C1.98669 8.46423 0.783562 6.03042 0.704375 2.78854C1.84406 2.71723 3.11325 2.32435 4.01538 1.99054C5.00762 1.62304 6.0095 1.15535 6.65 0.764666C7.2905 1.15535 8.29194 1.62304 9.28462 1.99054C10.1863 2.32523 11.4559 2.71723 12.5956 2.78854C12.5164 6.03042 11.3133 8.46423 10.2983 9.95304C9.06019 11.7695 7.59937 12.914 6.65 13.2789C5.70106 12.914 4.23981 11.7687 3.00169 9.95304ZM5.7022 8.64757C5.77045 8.71582 5.86014 8.74995 5.94983 8.74995H5.94895C6.0382 8.74995 6.12789 8.71626 6.19658 8.64757L9.69658 5.14757C9.83308 5.01107 9.83308 4.78926 9.69658 4.65276C9.56008 4.51626 9.33827 4.51626 9.20177 4.65276L5.94939 7.90513L4.79702 6.75276C4.66052 6.61626 4.4387 6.61626 4.3022 6.75276C4.1657 6.88926 4.1657 7.11107 4.3022 7.24757L5.7022 8.64757Z"
      />
    </SVGIcon>
  );
}
