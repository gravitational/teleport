/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

import PatchedJSDOMEnvironment from './jest-environment-patched-jsdom';

// jsdom captures the timezone when it builds the VM context, so a test file
// cannot set its own. Opt in with:
//
//   /**
//    * @jest-environment @gravitational/build/jest/jest-environment-time-zone
//    * @jest-environment-options {"timeZone": "America/Los_Angeles"}
//    */
export default class TimeZoneEnvironment extends PatchedJSDOMEnvironment {
  constructor(...args) {
    const [config] = args;
    const { timeZone } = config.projectConfig.testEnvironmentOptions;
    const previousTimeZone = process.env.TZ;
    if (timeZone) {
      process.env.TZ = timeZone;
    }
    super(...args);
    this.previousTimeZone = previousTimeZone;
  }

  async teardown() {
    // Test files share a worker process.
    process.env.TZ = this.previousTimeZone;
    await super.teardown();
  }
}

export const TestEnvironment = TimeZoneEnvironment;
