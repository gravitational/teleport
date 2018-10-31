/**
 * @license
 * Copyright (C) 2015 ribrdb @ code.google.com
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */


// Contributed by ribrdb @ code.google.com

/**
 * @fileoverview
 * Registers a language handler for YAML.
 *
 * @author ribrdb
 */

PR['registerLangHandler'](
  PR['createSimpleLexer'](
    [
      [PR['PR_PUNCTUATION'], /^[:|>?]+/, null, ':|>?'],
      [PR['PR_DECLARATION'],  /^%(?:YAML|TAG)[^#\r\n]+/, null, '%'],
      [PR['PR_TYPE'], /^[&]\S+/, null, '&'],
      [PR['PR_TYPE'], /^!\S*/, null, '!'],
      [PR['PR_STRING'], /^"(?:[^\\"]|\\.)*(?:"|$)/, null, '"'],
      [PR['PR_STRING'], /^'(?:[^']|'')*(?:'|$)/, null, "'"],
      [PR['PR_COMMENT'], /^#[^\r\n]*/, null, '#'],
      [PR['PR_PLAIN'], /^\s+/, null, ' \t\r\n']
    ],
    [
      [PR['PR_DECLARATION'], /^(?:---|\.\.\.)(?:[\r\n]|$)/],
      [PR['PR_PUNCTUATION'], /^-/],
      [PR['PR_KEYWORD'], /^[\w-]+:[ \r\n]/],
      [PR['PR_PLAIN'], /^\w+/]
    ]), ['yaml', 'yml']);

