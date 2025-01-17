/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import { Subtitle1 } from 'design';

import { ActionButtons, Header } from 'teleport/Discover/Shared';
import { Overview as IOverview } from 'teleport/Discover/Shared/Overview/types';
import { useDiscover } from 'teleport/Discover/useDiscover';

import { content } from './content';

export function Overview() {
  const { prevStep, nextStep, resourceSpec, isUpdateFlow } = useDiscover();

  if (!(resourceSpec.id in content)) {
    //todo handle error
  }

  let overview: IOverview = content[resourceSpec.id];

  return (
    <>
      <Header>Overview</Header>
      {overview.OverviewContent()}
      <Header>Prerequisites</Header>
      <Subtitle1>Make sure you have these ready before continuing.</Subtitle1>
      {overview.PrerequisiteContent()}
      <ActionButtons
        onProceed={nextStep}
        onPrev={isUpdateFlow ? null : prevStep}
      />
    </>
  );
}
