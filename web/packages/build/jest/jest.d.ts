/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Ignore no-empty-interface in order to explicitly follow the types from Jest docs.
// Otherwise ESLint would autofix them.
/* eslint-disable @typescript-eslint/no-empty-interface */

// https://jestjs.io/docs/27.x/expect#expectextendmatchers
// https://redd.one/blog/practical-guide-to-custom-jest-matchers
interface CustomMatchers<R = unknown> {
  toEventuallyBeTrue(args: { waitFor: number; tick: number }): Promise<R>;
}

declare global {
  namespace jest {
    interface Expect extends CustomMatchers {}
    interface Matchers<R> extends CustomMatchers<R> {}
    interface InverseAssymetricMatchers extends CustomMatchers {}
  }
}
export {};
