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

// makeLabelMaps makes a few lookup tables out of the label prop
// for easy lookup:
//  - lookup table with label.name as key, and label.value as value
//  - lookup table with label.value as key, and label.key as value
//  - lookup table of flags with label.name as key, and booleans as value
//    which serves to record seen labels.
export function makeLabelMaps(labels: AgentLabel[]) {
  let labelKeysToMatchMap: Record<string, string> = {};
  let labelValsToMatchMap: Record<string, string> = {};
  let labelToMatchSeenMap: Record<string, boolean> = {};

  labels.forEach(label => {
    labelKeysToMatchMap[label.name] = label.value;
    labelToMatchSeenMap[label.name] = false;
    labelValsToMatchMap[label.value] = label.name;
  });

  return { labelKeysToMatchMap, labelValsToMatchMap, labelToMatchSeenMap };
}

// matchLabels will go through each `matcherlabels` and record matched labels.
// If all labels are matched (or all asteriks exists in `matcherLabels`),
// returns true.
//
// It will return false when:
//   - there were no labels to match
//   - `matcherLabel` contains a label that isn't seen in the `xxxToMatchMap`
export function matchLabels({
  hasLabelsToMatch,
  matcherLabels,
  labelKeysToMatchMap,
  labelValsToMatchMap,
  labelToMatchSeenMap,
}: {
  hasLabelsToMatch: boolean;
  matcherLabels: Record<string, string[]>;
  labelKeysToMatchMap: Record<string, string>;
  labelValsToMatchMap: Record<string, string>;
  labelToMatchSeenMap: Record<string, boolean>;
}) {
  const matchedLabelMap = { ...labelToMatchSeenMap };
  // Sorted to have asteriks be the first label key to test.
  const entries = Object.entries({ ...matcherLabels }).sort();
  for (const [key, vals] of entries) {
    // Check if the label contains asteriks, which means match all eg:
    // a service with match all can pick up any database regardless of other labels
    // or no labels.
    const foundAsterikAsValue = vals.includes('*');
    if (key === '*' && foundAsterikAsValue) {
      return true;
    }

    if (!hasLabelsToMatch) {
      return false;
    }

    // Start matching by value.

    // This means any key is fine, as long as value matches.
    if (key === '*') {
      let found = false;
      vals.forEach(val => {
        const key = labelValsToMatchMap[val];
        if (key) {
          matchedLabelMap[key] = true;
          found = true;
        }
      });
      if (found) {
        continue;
      }
    }

    // This means any value is fine, as long as a key matches.
    // Note that db resource labels can't have duplicate keys
    // (but db service can).
    else if (foundAsterikAsValue && labelKeysToMatchMap[key]) {
      matchedLabelMap[key] = true;
      continue;
    }

    // Match against actual values of key and its value.
    else {
      const dbVal = labelKeysToMatchMap[key];
      if (dbVal && vals.find(val => val === dbVal)) {
        matchedLabelMap[key] = true;
        continue;
      }
    }

    // At this point, the current label did not match any criteria,
    // we can abort, since it takes only one mismatch to fail.
    return false;
  }

  return (
    hasLabelsToMatch &&
    Object.keys(matchedLabelMap).every(key => matchedLabelMap[key])
  );
}
