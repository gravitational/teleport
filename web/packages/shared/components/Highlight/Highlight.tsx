/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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
    autoEscape: true,
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
