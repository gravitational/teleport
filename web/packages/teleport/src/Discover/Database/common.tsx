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

import React from 'react';
import { Box, Label as Pill, Text } from 'design';
import * as Icons from 'design/Icon';

import { AgentLabel } from 'teleport/services/agents';
import { LabelsCreater, Mark, TextIcon } from 'teleport/Discover/Shared';
import { Regions } from 'teleport/services/integrations';

export const Labels = ({
  labels,
  setLabels,
  disableBtns = false,
  dbLabels,
  showLabelMatchErr = false,
  autoFocus = true,
  region,
}: {
  labels: AgentLabel[];
  setLabels(l: AgentLabel[]): void;
  disableBtns?: boolean;
  dbLabels: AgentLabel[];
  showLabelMatchErr?: boolean;
  autoFocus?: boolean;
  region?: Regions;
}) => {
  const hasDbLabels = dbLabels.length > 0;
  return (
    <Box mb={2}>
      <Text bold>Optionally Define Matcher Labels</Text>
      {!hasDbLabels && (
        <Text typography="subtitle1" mb={2}>
          Since no labels were defined for the registered database from the
          previous step, the matcher labels are defaulted to wildcards which
          will allow this database service to match any database.
        </Text>
      )}
      {hasDbLabels && (
        <>
          <Text typography="subtitle1" mb={2}>
            The default wildcard label allows this database service to match any
            database. If you're unsure about how label matching works in
            Teleport, leave this for now.
          </Text>
          <Text typography="subtitle1" mb={2}>
            Alternatively, you can define narrower labels that this database
            service will use to identify the databases you register
            {region ? (
              <span>
                {' '}
                in this region (<Mark>{region}</Mark>)
              </span>
            ) : (
              '.'
            )}{' '}
            In order to identify the database you registered in the previous
            step, the labels you define here must match with one of its existing
            labels:
          </Text>
          <Box mb={3}>
            {dbLabels.map((label, index) => {
              const labelText = `${label.name}: ${label.value}`;
              return (
                <Pill
                  key={`${label.name}${label.value}${index}`}
                  mr="1"
                  kind="secondary"
                >
                  {labelText}
                </Pill>
              );
            })}
          </Box>
        </>
      )}
      <LabelsCreater
        autoFocus={autoFocus}
        labels={labels}
        setLabels={setLabels}
        disableBtns={disableBtns || dbLabels.length === 0}
      />
      <Box mt={1} mb={3}>
        {showLabelMatchErr && (
          <TextIcon>
            <Icons.Warning ml={1} color="error.main" />
            The matcher labels must be able to match with the labels defined for
            the registered database. Use wildcards to match with any labels.
          </TextIcon>
        )}
      </Box>
    </Box>
  );
};

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

// hasMatchingLabels will go through each 'agentLabels' and find matches from
// 'dbLabels'. The 'agentLabels' must have same amount of matching labels
// with 'dbLabels' either with asteriks (match all) or by exact match.
//
// `agentLabels` have OR comparison eg:
//  - If agent labels was defined like this [`fruit: apple`, `fruit: banana`]
//    it's translated as `fruit: [apple OR banana]`.
//
// asterisks can be used for keys, values, or both key and value eg:
//  - `fruit: *` match by key `fruit` with any value
//  - `*: apple` match by value `apple` with any key
//  - `*: *` match by any key and any value
export function hasMatchingLabels(
  dbLabels: AgentLabel[],
  agentLabels: AgentLabel[]
) {
  // Convert agentLabels into a map of key of value arrays.
  const matcherLabels: Record<string, string[]> = {};
  agentLabels.forEach(l => {
    if (!matcherLabels[l.name]) {
      matcherLabels[l.name] = [];
    }
    matcherLabels[l.name] = [...matcherLabels[l.name], l.value];
  });

  return matchLabels(dbLabels, matcherLabels);
}
