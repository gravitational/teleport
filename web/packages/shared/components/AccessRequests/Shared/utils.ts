/**
 * Copyright 2024 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { formatDuration, intervalToDuration } from 'date-fns';

import { ResourceMap } from '../NewRequest';

export function getFormattedDurationTxt({
  start,
  end,
}: {
  start: Date;
  end: Date;
}) {
  return formatDuration(intervalToDuration({ start, end }), {
    format: ['weeks', 'days', 'hours', 'minutes'],
  });
}

export function getNumAddedResources(addedResources: ResourceMap) {
  return (
    Object.keys(addedResources.node).length +
    Object.keys(addedResources.db).length +
    Object.keys(addedResources.app).length +
    Object.keys(addedResources.kube_cluster).length +
    Object.keys(addedResources.user_group).length +
    Object.keys(addedResources.windows_desktop).length
  );
}
