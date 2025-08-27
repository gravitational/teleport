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

export function getConnectCsp(development: boolean) {
  // feedbackAddress needs to be kept in sync with the same property in staticConfig.ts.
  const feedbackAddress = development
    ? 'https://kcwm2is93l.execute-api.us-west-2.amazonaws.com/prod'
    : 'https://usage.teleport.dev';

  const scriptEval = development
    ? // Required to make source maps work in dev mode.
      "'unsafe-eval' 'unsafe-inline'"
    : // Enables WASM initialization with a safer alternative to 'unsafe-eval'.
      // This source expression applies specifically to WASM.
      "'wasm-unsafe-eval'";

  return `
default-src 'self';
connect-src 'self' ${feedbackAddress};
style-src 'self' 'unsafe-inline';
img-src 'self' data: blob:;
object-src 'none';
font-src 'self' data:;
script-src 'self' ${scriptEval};
`
    .replaceAll('\n', ' ')
    .trim();
}
