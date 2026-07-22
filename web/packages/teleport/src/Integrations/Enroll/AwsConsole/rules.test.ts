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

import { validTrustAnchorInput } from 'teleport/Integrations/Enroll/AwsConsole/rules';

const excess = `
  4. Create a Roles Anywhere Profile in AWS IAM for your Teleport cluster.
CreateRolesAnywhereProfileProvider: {
    "Name": "MarcoRAProfileFromCLI",
    "RoleArns": [
        "arn:aws:iam::123456789012:role/MarcoRARoleFromCLI"
    ],

Copy and paste the following values to Teleport UI

=================================================
arn:aws:rolesanywhere:eu-west-2:123456789012:trust-anchor/00000000-0000-0000-0000-000000000000
arn:aws:rolesanywhere:eu-west-2:123456789012:profile/00000000-0000-0000-0000-000000000000
arn:aws:iam::123456789012:role/MarcoRARoleFromCLI
=================================================

2025-05-15T16:30:21.683+01:00 INFO  Success! operation:awsra-trust-anchor provisioning/operations.go:190
`;
const topExcess = `

=================================================
arn:aws:rolesanywhere:eu-west-2:123456789012:trust-anchor/00000000-0000-0000-0000-000000000000
arn:aws:rolesanywhere:eu-west-2:123456789012:profile/00000000-0000-0000-0000-000000000000
arn:aws:iam::123456789012:role/MarcoRARoleFromCLI
`;
const bottomExcess = `
arn:aws:rolesanywhere:eu-west-2:123456789012:trust-anchor/00000000-0000-0000-0000-000000000000
arn:aws:rolesanywhere:eu-west-2:123456789012:profile/00000000-0000-0000-0000-000000000000
arn:aws:iam::123456789012:role/MarcoRARoleFromCLI
=================================================
`;
const perfect = `
arn:aws:rolesanywhere:eu-west-2:123456789012:trust-anchor/00000000-0000-0000-0000-000000000000
arn:aws:rolesanywhere:eu-west-2:123456789012:profile/00000000-0000-0000-0000-000000000000
arn:aws:iam::123456789012:role/MarcoRARoleFromCLI
`;
const missingArn = `
:aws:rolesanywhere:eu-west-2:123456789012:trust-anchor/00000000-0000-0000-0000-000000000000
arn:aws:rolesanywhere:eu-west-2:123456789012:profile/00000000-0000-0000-0000-000000000000
arn:aws:iam::123456789012:role/MarcoRARoleFromCLI
`;
const missingTrust = `
arn:aws:rolesanywhere:eu-west-2:123456789012:nottheanchor/00000000-0000-0000-0000-000000000000
arn:aws:rolesanywhere:eu-west-2:123456789012:profile/00000000-0000-0000-0000-000000000000
arn:aws:iam::123456789012:role/MarcoRARoleFromCLI
`;
const missingProfile = `
arn:aws:rolesanywhere:eu-west-2:123456789012:trust-anchor/00000000-0000-0000-0000-000000000000
arn:aws:rolesanywhere:eu-west-2:123456789012:nottheprofile/00000000-0000-0000-0000-000000000000
arn:aws:iam::123456789012:role/MarcoRARoleFromCLI
`;
const missingRole = `
arn:aws:rolesanywhere:eu-west-2:123456789012:trust-anchor/00000000-0000-0000-0000-000000000000
arn:aws:rolesanywhere:eu-west-2:123456789012:profile/00000000-0000-0000-0000-000000000000
arn:aws:iam::123456789012:nottherole/MarcoRARoleFromCLI
`;
const missingAll = `
foo
bar
baz
`;

test.each`
  name                          | input             | valid    | message
  ${'valid excess copy'}        | ${excess}         | ${true}  | ${undefined}
  ${'valid excess top copy'}    | ${topExcess}      | ${true}  | ${undefined}
  ${'valid excess bottom copy'} | ${bottomExcess}   | ${true}  | ${undefined}
  ${'valid perfect copy'}       | ${perfect}        | ${true}  | ${undefined}
  ${'invalid missing arn'}      | ${missingArn}     | ${false} | ${'Each line should start with arn:aws:, please double check the output'}
  ${'invalid missing trust'}    | ${missingTrust}   | ${false} | ${'Output is missing trust anchor, please double check the output'}
  ${'invalid missing profile'}  | ${missingProfile} | ${false} | ${'Output is missing profile, please double check the output'}
  ${'invalid missing role'}     | ${missingRole}    | ${false} | ${'Output is missing role, please double check the output'}
  ${'invalid missingAll'}       | ${missingAll}     | ${false} | ${'Each line should start with arn:aws:, please double check the output'}
`(`parseOutput $name`, ({ input, valid, message }) => {
  const validationResult = validTrustAnchorInput(input)();
  expect(validationResult.valid).toBe(valid);
  expect(validationResult.message).toBe(message);
});
