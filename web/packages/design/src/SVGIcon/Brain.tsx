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

export function BrainIcon({ size = 22, fill }: SVGIconProps) {
  return (
    <SVGIcon fill={fill} size={size} viewBox="0 0 20 20">
      <path d="M12.5 7a.5.5 0 0 1 0-1c.276 0 .5-.224.5-.5a.5.5 0 0 1 1 0c0 .827-.673 1.5-1.5 1.5zM12.585 15a.499.499 0 0 1-.471-.333 2.002 2.002 0 0 1 2.553-2.553.5.5 0 0 1-.333.943 1.002 1.002 0 0 0-1.276 1.276.5.5 0 0 1-.472.667z" />
      <path d="M19 10.5c0-2.459-.764-4.801-2.21-6.779a3.998 3.998 0 0 0-2.339-2.45 3 3 0 0 0-4.951.072 3 3 0 0 0-4.951-.072 3.993 3.993 0 0 0-2.339 2.45A11.406 11.406 0 0 0 0 10.5c0 .655.056 1.312.166 1.953A7.175 7.175 0 0 0 0 14c0 1.584.508 3.078 1.431 4.206C2.378 19.363 3.645 20 5 20c.263 0 .527-.025.785-.073A3.501 3.501 0 0 0 9.5 18.301a3.501 3.501 0 0 0 3.715 1.626c.259.049.522.073.785.073 1.355 0 2.622-.637 3.569-1.794C18.492 17.078 19 15.584 19 14c0-.531-.056-1.049-.166-1.547.11-.641.166-1.297.166-1.953zM6.5 19A2.503 2.503 0 0 1 4 16.5a.5.5 0 0 0-1 0c0 .876.323 1.677.857 2.292C2.207 18.176 1 16.261 1 14c0-1.158.326-2.288.917-3.181a.5.5 0 0 0-.834-.552l-.082.127a10.429 10.429 0 0 1 1.087-4.557 3.976 3.976 0 0 0 1.055 1.962.499.499 0 1 0 .714-.7 2.998 2.998 0 0 1 .153-4.345 3.015 3.015 0 0 0 .05.846.5.5 0 0 0 .98-.198A2.002 2.002 0 0 1 7 1.001c1.103 0 2 .897 2 2v6.766a2.987 2.987 0 0 0-2.604-.705.5.5 0 0 0 .2.98A2.002 2.002 0 0 1 9 12.001v4.5c0 1.378-1.122 2.5-2.5 2.5zm8.643-.209A3.486 3.486 0 0 0 16 16.5a.5.5 0 0 0-1 0c0 1.378-1.122 2.5-2.5 2.5S10 17.878 10 16.5V12a2.002 2.002 0 0 1 2.404-1.959.5.5 0 0 0 .2-.98A2.987 2.987 0 0 0 10 9.766V3c0-1.103.897-2 2-2a2.002 2.002 0 0 1 1.96 2.401.499.499 0 1 0 .98.198 3.005 3.005 0 0 0 .05-.845 3 3 0 0 1 .153 4.345.5.5 0 0 0 .714.7 3.972 3.972 0 0 0 1.055-1.962 10.425 10.425 0 0 1 1.087 4.557 5.51 5.51 0 0 0-.082-.127.5.5 0 0 0-.834.552c.591.893.917 2.023.917 3.181 0 2.26-1.206 4.175-2.857 4.791z" />
      <path d="M6.5 7C5.673 7 5 6.327 5 5.5a.5.5 0 0 1 1 0c0 .276.224.5.5.5a.5.5 0 0 1 0 1zM6.415 15a.5.5 0 0 1-.472-.667 1.002 1.002 0 0 0-1.276-1.276.5.5 0 1 1-.333-.943 2.002 2.002 0 0 1 2.553 2.553.5.5 0 0 1-.471.333z" />
    </SVGIcon>
  );
}
