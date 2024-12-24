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

import React, {useEffect} from 'react';
import Table from 'design/DataTable';

import {useAsync} from "shared/hooks/useAsync";

import {useParams} from "react-router";

import {Indicator} from "design";

import {Danger} from "design/Alert";

import { FeatureBox } from 'teleport/components/Layout';
import { AwsOidcHeader } from 'teleport/Integrations/status/AwsOidc/AwsOidcHeader';
import { useAwsOidcStatus } from 'teleport/Integrations/status/AwsOidc/useAwsOidcStatus';
import {IntegrationKind, integrationService, UserTask} from "teleport/services/integrations";


export function Tasks() {
    const { name } = useParams<{
        type: IntegrationKind;
        name: string;
    }>();

  const { integrationAttempt } = useAwsOidcStatus();
  const { data: integration } = integrationAttempt;

    const [attempt, fetchTasks] = useAsync(() =>
        integrationService.fetchIntegrationUserTasksList(name)
    );

    useEffect(() => {
        fetchTasks();
    }, []);

    if (attempt.status == 'processing') {
        return <Indicator />;
    }

    if (attempt.status == 'error') {
        return <Danger>{attempt.statusText}</Danger>;
    }

    if (!attempt.data) {
        return null;
    }

  return (
    <FeatureBox css={{ maxWidth: '1400px', paddingTop: '16px', gap: '30px' }}>
      {integration && <AwsOidcHeader integration={integration} tasks={true} />}
      {/*  todo (michellescripts) sync with Marco on timestamp field */}
      <Table<UserTask>
        data={attempt.data.items}
        columns={[
          {
            key: 'taskType',
            headerText: 'Type',
            isSortable: true,
          },
          {
            key: 'name',
            headerText: 'Issue Details',
            isSortable: true,
          },
        ]}
        emptyText={`No pending tasks`}
        isSearchable
      />
    </FeatureBox>
  );
}
