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

import { InfoGuideTab } from 'teleport/Integrations/Enroll/Cloud/Shared/InfoGuide';
import {
  IntegrationKind,
  IntegrationWithSummary,
} from 'teleport/services/integrations';

import { SettingsTab as AwsSettingsTab } from './Aws/SettingsTab';
import { SettingsTab as AzureSettingsTab } from './Azure/SettingsTab';

export function SettingsTab({
  stats,
  activeInfoGuideTab,
  onInfoGuideTabChange,
}: {
  stats: IntegrationWithSummary;
  activeInfoGuideTab: InfoGuideTab | null;
  onInfoGuideTabChange: (tab: InfoGuideTab) => void;
}) {
  switch (stats.subKind) {
    case IntegrationKind.AwsOidc:
      return (
        <AwsSettingsTab
          stats={stats}
          activeInfoGuideTab={activeInfoGuideTab}
          onInfoGuideTabChange={onInfoGuideTabChange}
        />
      );
    case IntegrationKind.AzureOidc:
      return (
        <AzureSettingsTab
          stats={stats}
          activeInfoGuideTab={activeInfoGuideTab}
          onInfoGuideTabChange={onInfoGuideTabChange}
        />
      );
  }
}
