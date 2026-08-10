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

import { Meta, StoryObj } from '@storybook/react-vite';
import { http, HttpResponse } from 'msw';
import { Routes, Route } from 'react-router';

import cfgE from 'e-teleport/config';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { TeleportProviderBasic } from 'teleport/mocks/providers';

import {
  ColumnType,
  GetReportResponse,
  GetReportStateResponse,
  ReportState,
} from '../types';
import { Report } from './Report';

const meta = {
  title: 'TeleportE/AccessMonitoring/Reports',
  component: Wrapper,
} satisfies Meta<typeof Wrapper>;

type Story = StoryObj<typeof meta>;

export default meta;

export const MaxResults: Story = {
  beforeEach({ msw }) {
    msw.use(
      http.get(cfgE.api.accessMonitoring.reportState, () => {
        return HttpResponse.json({
          status: ReportState.Ready,
        } satisfies GetReportStateResponse);
      }),
      http.get(cfgE.api.accessMonitoring.reportResult, () => {
        return HttpResponse.json({
          name: 'privilege_access_report',
          description: 'Lorem ipsum delor sit amet',
          updated_at: new Date().toISOString(),
          audit_query_results: [
            {
              result: {
                column_info: [
                  {
                    name: 'event_date',
                    type: ColumnType.Date,
                  },
                  {
                    name: 'user',
                    type: ColumnType.VarChar,
                  },
                  {
                    name: 'count',
                    type: ColumnType.BigInt,
                  },
                ],
                rows: generateSampleRows({
                  name: 'cert_expiration_more_than_1d',
                  days: 40,
                  users: 25,
                }),
              },
              audit_query: {
                name: 'cert_expiration_more_than_1d',
                title: 'Long Lived Certificates',
                description: 'Lorem ipsum delor sit amet',
                query: "select 'mocked query';",
              },
            },
          ],
        } satisfies GetReportResponse);
      })
    );
  },
};

function Wrapper(props: { name?: string; days?: number }) {
  const { name = 'privilege_access_report', days = 30 } = props;
  const ctx = createTeleportContext();

  return (
    <TeleportProviderBasic
      teleportCtx={ctx}
      initialEntries={[cfgE.getAccessMonitoringReportRoute(name, days)]}
    >
      <Routes>
        <Route
          path={cfgE.routes.accessMonitoring.report}
          element={<Report />}
        />
      </Routes>
    </TeleportProviderBasic>
  );
}

function generateSampleRows(config: {
  name: 'cert_expiration_more_than_1d';
  days: number;
  users: number;
}) {
  if (config.name === 'cert_expiration_more_than_1d') {
    const days = Array.from({ length: config.days }, (_, i) => {
      const date = new Date();
      date.setDate(date.getDate() - config.days + 1 + i);
      return date;
    });
    const users = Array.from(
      { length: config.users },
      (_, i) => `user-${i + 1}`
    );

    return [
      {
        data: ['event_date', 'user', 'count'],
      },
      ...days.flatMap(day => {
        return users.map(user => {
          return {
            data: [day.toISOString(), user, '1'],
          };
        });
      }),
    ];
  }
}
