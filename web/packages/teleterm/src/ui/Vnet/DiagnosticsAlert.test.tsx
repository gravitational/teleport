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

import { useEffect } from 'react';

import { render, screen } from 'design/utils/testing';
import {
  CheckAttemptStatus,
  CheckReportStatus,
  Report,
} from 'gen-proto-ts/teleport/lib/vnet/diag/v1/diag_pb';

import { MockedUnaryCall } from 'teleterm/services/tshd/cloneableClient';
import {
  makeCheckAttempt,
  makeCheckReport,
  makeReport,
} from 'teleterm/services/vnet/testHelpers';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { ConnectionsContextProvider } from 'teleterm/ui/TopBar/Connections/connectionsContext';

import { DiagnosticsAlert } from './DiagnosticsAlert';
import { useVnetContext, VnetContextProvider } from './vnetContext';

const noIssuesFound = 'No issues found';
const otherSoftwareMightInterfere =
  'software on your device might interfere with VNet';
const someChecksFailed = 'Some diagnostic checks failed';

describe('DiagnosticsAlert', () => {
  const tests: Array<{
    when: string;
    expectedMessage: string;
    makeReport: () => Report;
  }> = [
    {
      when: 'all checks succeeded and have found no issues',
      expectedMessage: noIssuesFound,
      makeReport,
    },
    {
      when: 'all checks found issues',
      expectedMessage: otherSoftwareMightInterfere,
      makeReport: () =>
        makeReport({
          checks: [
            makeCheckAttempt({
              checkReport: makeCheckReport({
                status: CheckReportStatus.ISSUES_FOUND,
              }),
            }),
          ],
        }),
    },
    {
      when: 'all checks failed to complete',
      expectedMessage: someChecksFailed,
      makeReport: () =>
        makeReport({
          checks: [
            makeCheckAttempt({
              status: CheckAttemptStatus.ERROR,
              error: 'something went wrong',
              checkReport: undefined,
            }),
          ],
        }),
    },
    {
      when: 'some checks found no issues, some did',
      expectedMessage: otherSoftwareMightInterfere,
      makeReport: () =>
        makeReport({
          checks: [
            makeCheckAttempt({
              checkReport: makeCheckReport({
                status: CheckReportStatus.OK,
              }),
            }),
            makeCheckAttempt({
              checkReport: makeCheckReport({
                status: CheckReportStatus.ISSUES_FOUND,
              }),
            }),
          ],
        }),
    },
    {
      when: 'some checks found no issues, some failed to complete',
      expectedMessage: someChecksFailed,
      makeReport: () =>
        makeReport({
          checks: [
            makeCheckAttempt({
              checkReport: makeCheckReport({
                status: CheckReportStatus.OK,
              }),
            }),
            makeCheckAttempt({
              status: CheckAttemptStatus.ERROR,
              error: 'something went wrong',
              checkReport: undefined,
            }),
          ],
        }),
    },
    {
      when: 'some checks found issues, some failed to complete',
      expectedMessage: otherSoftwareMightInterfere,
      makeReport: () =>
        makeReport({
          checks: [
            makeCheckAttempt({
              checkReport: makeCheckReport({
                status: CheckReportStatus.ISSUES_FOUND,
              }),
            }),
            makeCheckAttempt({
              status: CheckAttemptStatus.ERROR,
              error: 'something went wrong',
              checkReport: undefined,
            }),
          ],
        }),
    },
    {
      when: 'some checks found no issues, some failed to complete',
      expectedMessage: someChecksFailed,
      makeReport: () =>
        makeReport({
          checks: [
            makeCheckAttempt({
              checkReport: makeCheckReport({
                status: CheckReportStatus.OK,
              }),
            }),
            makeCheckAttempt({
              status: CheckAttemptStatus.ERROR,
              error: 'something went wrong',
              checkReport: undefined,
            }),
          ],
        }),
    },
  ];

  test.each(tests)(
    'when $when it says "$expectedMessage"',
    async ({ makeReport, expectedMessage }) => {
      const appContext = new MockAppContext();
      const report = makeReport();
      appContext.vnet.runDiagnostics = () => new MockedUnaryCall({ report });

      render(
        <MockAppContextProvider appContext={appContext}>
          <ConnectionsContextProvider>
            <VnetContextProvider>
              <RunDiagnostics />
              <DiagnosticsAlert />
            </VnetContextProvider>
          </ConnectionsContextProvider>
        </MockAppContextProvider>
      );

      expect(
        await screen.findByText(new RegExp(expectedMessage))
      ).toBeInTheDocument();
    }
  );
});

const RunDiagnostics = () => {
  const { runDiagnostics } = useVnetContext();
  useEffect(() => {
    void runDiagnostics();
  }, [runDiagnostics]);

  return null;
};
