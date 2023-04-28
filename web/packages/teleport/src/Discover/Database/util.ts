/**
 * Copyright 2023 Gravitational, Inc.
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

import type { AgentLabel } from 'teleport/services/agents';

export function matchLabels(
  newDbLabels: AgentLabel[],
  matcherLabels: Record<string, string[]>
) {
  // Sorting to match by asteriks sooner.
  const entries = Object.entries({ ...matcherLabels }).sort();

  if (!entries.length) {
    return false;
  }

  // Create a map for db labels for easy lookup.
  let dbKeyMap = {};
  let dbValMap = {};

  newDbLabels.forEach(label => {
    dbKeyMap[label.name] = label.value;
    dbValMap[label.value] = label.name;
  });

  // All matching labels must make a match with the new database labels.
  let matched = true;
  for (let i = 0; i < entries.length; i++) {
    const [key, vals] = entries[i];
    // Check if this label set contains asteriks, which means match all.
    // A service with match all can pick up any database regardless of other labels
    // or no labels.
    const foundAsterikAsValue = vals.includes('*');
    if (key === '*' && foundAsterikAsValue) {
      return true;
    }

    // If no newDbLabels labels were defined, there are no matches to make,
    // which makes this service not a match.
    if (!newDbLabels.length) {
      matched = false;
      break;
    }

    // Start matching by values.

    // This means any key is fine, as long as a value matches.
    if (key === '*' && vals.find(val => dbValMap[val])) {
      continue;
    }

    // This means any value is fine, as long as a key matches.
    if (foundAsterikAsValue && dbKeyMap[key]) {
      continue;
    }

    // Match against key and value.
    const dbVal = dbKeyMap[key];
    if (dbVal && vals.find(val => val === dbVal)) {
      continue;
    }

    // No matches were found for this label set which
    // means this service not a match.
    matched = false;
    break;
  } // label set loop

  return matched;
}
