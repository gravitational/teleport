/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

/**
 * This needs to be an .mjs file,
 * because Vite can't import .ts files from a workspace out of the box.
 * https://github.com/vitejs/vite/issues/5370
 */

import react from '@vitejs/plugin-react-swc';

/** @param {string} mode */
export function reactPlugin(mode) {
  return react({
    plugins: [['@swc/plugin-styled-components', getStyledComponentsConfig(mode)]],
  });
}

/** @param {string} mode */
function getStyledComponentsConfig(mode) {
  // https://nextjs.org/docs/advanced-features/compiler#styled-components
  if (mode === 'production') {
    return {
      ssr: false,
      pure: false, // not currently supported by SWC
      displayName: false,
      fileName: false,
      cssProp: true,
    };
  }

  return {
    ssr: false,
    pure: true, // not currently supported by SWC
    displayName: true,
    fileName: true,
    cssProp: true,
  };
}
