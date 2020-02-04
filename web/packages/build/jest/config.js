/*
Copyright 2020 Gravitational, Inc.

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

const path = require('path');

module.exports = {
  moduleNameMapper: {
    // mock all imports to asset files
    '\\.(css|scss|stylesheet)$': path.join(__dirname, 'mockStyles.js'),
    '\\.(png|svg)$': path.join(__dirname, 'mockFiles.js'),
    // Below aliases allow easier migration of gravitational code to this monorepo.
    // They also give shorter names to gravitational packages.
    jQuery: 'jquery',
    '^shared/(.*)$': '<rootDir>/packages/shared/$1',
    '^design($|/.*)': '<rootDir>/packages/design/src/$1',
    '^gravity/(.*)$': '<rootDir>/packages/gravity/src/$1',
    '^teleport/(.*)$': '<rootDir>/packages/teleport/src/$1',
    '^e-teleport/(.*)$': '<rootDir>/packages/webapps.e/teleport/src/$1',
    '^e-shared/(.*)$': '<rootDir>/packages/webapps.e/shared/src/$1',
    '^e-gravity/(.*)$': '<rootDir>/packages/webapps.e/gravity/src/$1',
  },
};
