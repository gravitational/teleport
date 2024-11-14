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
import React, { useEffect } from 'react';
import { useAsync } from 'shared/hooks/useAsync';
import { Flex, H1, H2, H3, H4, H5, H6, Text, Box } from 'design';
import { SyncAlt } from 'design/Icon';
import { ToolTipInfo } from 'shared/components/ToolTip';
import * as Icons from 'design/Icon';

import { Integration } from 'teleport/services/integrations';
import { useTeleport } from 'teleport/index';

import {
  Panel,
  PanelTitle,
  CenteredSpaceBetweenFlex,
  CustomLabel,
  ErrorTooltip,
  InnerCard,
  GappedColumnFlex,
  PanelHeader,
  PanelLastSynced,
} from '../../Shared';

import { PanelIcon } from '../../getResourceIcon';

export function ListEksClusters() {
  const ctx = useTeleport();
  // check access?

  const [attempt, fetchEc2Instances] = useAsync(async () => {
    // await some listing
  });

  useEffect(() => {
    // has access, fetch ec2 instances
  }, []);

  return <>eks CLUSTER table</>;
}
