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

import React from 'react';
import { findAll } from 'highlight-words-core';

/**
 * Highlight wraps the keywords found in the text in <mark> tags.
 *
 * It is a simplified version of the component provided by the react-highlight-words package.
 * It can be extended with the features provided by highlight-words-core (e.g. case sensitivity).
 *
 * It doesn't handle Unicode super well because highlight-words-core uses a regex with the i flag
 * underneath. This means that the component will not always ignore differences in case, for example
 * when matching a string with the Turkish İ.
 */
export function Highlight(props: { text: string; keywords: string[] }) {
  const chunks = findAll({
    textToHighlight: props.text,
    searchWords: props.keywords,
  });

  return (
    <>
      {chunks.map((chunk, index) => {
        const { end, highlight, start } = chunk;
        const chunkText = props.text.substr(start, end - start);

        if (highlight) {
          return <mark key={index}>{chunkText}</mark>;
        } else {
          return chunkText;
        }
      })}
    </>
  );
}
