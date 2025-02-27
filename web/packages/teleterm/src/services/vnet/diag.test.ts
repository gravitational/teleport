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

import * as diag from 'gen-proto-ts/teleport/lib/vnet/diag/v1/diag_pb';

import { reportToText } from './diag';
import {
  makeCheckAttempt,
  makeCheckReport,
  makeCommandAttempt,
  makeReport,
  makeRouteConflict,
} from './testHelpers';

describe('reportToText', () => {
  it('converts report correctly', () => {
    const checkReport = makeCheckReport({
      status: diag.CheckReportStatus.ISSUES_FOUND,
    });
    checkReport.report = {
      oneofKind: 'routeConflictReport',
      routeConflictReport: {
        routeConflicts: [
          makeRouteConflict({
            dest: '100.64.0.0/10',
            vnetDest: '100.64.0.1',
            interfaceName: 'utun5',
            interfaceApp: 'VPN: Foobar',
          }),
          makeRouteConflict({
            dest: '0.0.0.0/1',
            vnetDest: '100.64.0.0/10',
            interfaceName: 'utun6',
            interfaceApp: '',
          }),
        ],
      },
    };
    const report = makeReport({
      checks: [
        makeCheckAttempt({
          checkReport,
          commands: [makeCommandAttempt()],
        }),
      ],
    });

    const actualText = reportToText(report);
    expect(actualText).toMatchSnapshot();
    // Verify that the text ends with a newline.
    expect(actualText.endsWith('\n')).toBe(true);
  });
});
