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

import fs from 'node:fs';

import {
  BASE_RESOURCES,
  SAML_APPLICATIONS,
} from 'teleport/Discover/SelectResource/resources';

import { createGuidedResourceList } from './gen-guided-resources.js';

const header = `{/* Generated file. Do not edit. */}
{/* To regenerate, run: make web-ui-docs */}

<style dangerouslySetInnerHTML={{__html: \`
  /*
    Since the list of supported Web UI enrollment flows includes multiple
    tables, set a fixed cell width to ensure that all tables are aligned.
  */
  table {
    table-layout: fixed;
  }

  th, td {
    width: 125px;
  }

  th:first-child, td:first-child {
    width: 400px;
  }
  \`
  }}
/>`;

if (process.argv.length !== 3) {
  console.error(
    'The argument of the script must be the path of the output file.'
  );
  process.exit(1);
}

const outputPath = process.argv[2];
console.log('Writing guided resource list to', outputPath);

fs.writeFileSync(
  outputPath,
  `${header}\n\n${createGuidedResourceList([...BASE_RESOURCES, ...SAML_APPLICATIONS])}
`
);
