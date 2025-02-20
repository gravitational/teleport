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

import { Timestamp } from 'gen-proto-ts/google/protobuf/timestamp_pb';
import {
  CheckAttempt,
  CheckAttemptStatus,
  CheckReport,
  CheckReportStatus,
  Report,
  RouteConflict,
} from 'gen-proto-ts/teleport/lib/vnet/diag/v1/diag_pb';

export const makeReport = (props: Partial<Report> = {}): Report => ({
  createdAt: Timestamp.fromDate(new Date(2024, 10, 23, 13, 27, 48)),
  checks: [makeCheckAttempt()],
  networkStackAttempt: {
    status: CheckAttemptStatus.OK,
    error: '',
    networkStack: {
      dnsZones: ['teleport.example.com', 'company.test'],
      interfaceName: 'utun4',
      ipv4CidrRanges: ['100.64.0.0/10'],
      ipv6Prefix: 'fdff:fd74:46c0::',
    },
  },
  ...props,
});

export const makeCheckAttempt = (
  props: Partial<CheckAttempt> = {}
): CheckAttempt => ({
  status: CheckAttemptStatus.OK,
  error: '',
  commands: [],
  checkReport: makeCheckReport(),
  ...props,
});

export const makeCheckReport = (
  props: Partial<CheckReport> = {}
): CheckReport => ({
  status: CheckReportStatus.OK,
  report: {
    oneofKind: 'routeConflictReport',
    routeConflictReport: {
      routeConflicts: [],
    },
  },
  ...props,
});

export const makeRouteConflict = (
  props: Partial<RouteConflict> = {}
): RouteConflict => ({
  dest: '100.64.0.0/10',
  vnetDest: '100.64.0.1',
  interfaceName: 'utun5',
  interfaceApp: '',
  ...props,
});
