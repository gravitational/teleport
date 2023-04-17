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

// If you ever need to pluralize a word which cannot be pluralized by appending 's', just add a
// third optional argument which is the pluralized noun.
// https://api.rubyonrails.org/v7.0.4.2/classes/ActionView/Helpers/TextHelper.html#method-i-pluralize

/**
 * pluralize adds an 's' to the given word if num is bigger than 1.
 */
export function pluralize(num: number, word: string) {
  if (num > 1) {
    return `${word}s`;
  }

  return word;
}
