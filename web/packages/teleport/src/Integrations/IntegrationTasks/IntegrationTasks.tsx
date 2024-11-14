/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import React, { useEffect, PropsWithChildren, useState } from 'react';
import { useHistory, Link } from 'react-router-dom';
import { Indicator, Box, Alert, Flex, ButtonIcon, Text, Label } from 'design';
import useAttempt from 'shared/hooks/useAttemptNext';
import { useParams } from 'react-router';
import { ArrowLeft } from 'design/Icon';
import { HoverTooltip } from 'shared/components/ToolTip';
import { ResourceIcon } from 'design/ResourceIcon';

import { FeatureBox } from 'teleport/components/Layout';
import {
  IntegrationKind,
  IntegrationStatusCode,
  PluginKind,
  PluginStatus,
} from 'teleport/services/integrations';
import cfg from 'teleport/config';
import { AwsPendingTasks } from './AwsPendingTasks';

export function IntegrationTasks() {
  const { type, name } = useParams<{
    type: PluginKind | IntegrationKind;
    name: string;
  }>();

  if (type === IntegrationKind.AwsOidc) {
    // TODO Permission check?
    return <AwsPendingTasks integrationName={name} />;
  }

  return <>tasks for {type} not implemented</>;
}
