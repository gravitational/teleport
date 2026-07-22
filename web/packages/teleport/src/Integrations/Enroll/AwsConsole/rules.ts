/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { ValidationResult } from 'shared/components/Validation/rules';

/**
 * Returns the validity of the trust anchor input including trust anchor, profile and role arns
 *
 * @remarks
 * Loosely checks the inputs for validity; the API will do further validations using the aws package.
 * Will return invalid if there are not three lines that begin with arn or if one of the following is missing: trust anchor, profile, role.
 *
 * @param value - The target to validate
 * @returns () => {valid: boolean, message: string}
 *
 */
export const validTrustAnchorInput =
  (value: string) => (): ValidationResult => {
    if (!value) {
      return {
        valid: true,
      };
    }

    let lines = value.split('\n');
    const startsWithArn = lines.filter(l => l.startsWith('arn:aws:'));
    if (startsWithArn.length < 3) {
      return {
        valid: false,
        message:
          'Each line should start with arn:aws:, please double check the output',
      };
    }

    const trustAnchor = lines.find(l => l.includes(':trust-anchor/'));
    if (!trustAnchor) {
      return {
        valid: false,
        message:
          'Output is missing trust anchor, please double check the output',
      };
    }

    const profile = lines.find(l => l.includes(':profile/'));
    if (!profile) {
      return {
        valid: false,
        message: 'Output is missing profile, please double check the output',
      };
    }
    const role = lines.find(l => l.includes(':role/'));
    if (!role) {
      return {
        valid: false,
        message: 'Output is missing role, please double check the output',
      };
    }

    return { valid: true };
  };
