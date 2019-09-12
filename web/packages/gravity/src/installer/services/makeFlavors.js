/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import { at, map, keys, keyBy } from 'lodash';

export default function makeFlavors(flavorsJson, app, operation){
  // create a map from node profile array
  const nodeProfiles = keyBy(app.nodeProfiles, i => i.name);

  const [ flavorItems, defaultFlavor, prompt] = at(flavorsJson, [
    'items',
    'default',
    'title',
  ]);

  // flavor options
  const options = map(flavorItems, item => {
    const { description, name, profiles: flavorProfiles } = item;

    // create new profile objects from node profiles, operation agent, and flavor profiles
    const profiles = keys(flavorProfiles).map(key => {
      const {
        instance_types,
        instance_type,
        count
      } = flavorProfiles[key];

      const { description, requirementsText } = nodeProfiles[key];

      const instructions = at(operation, `details.agents.${key}.instructions`)[0];

      return {
        name: key,
        instanceTypes: instance_types,
        instanceTypeFixed: instance_type,
        instructions,
        count,
        description,
        requirementsText
      }
    });

    return {
      name,
      isDefault: name === defaultFlavor,
      title: description || name,
      profiles
    }
  });

  return {
    options,
    prompt,
  }
}
