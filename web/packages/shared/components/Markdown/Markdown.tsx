/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { createElement, useMemo, type ReactNode } from 'react';
import styled from 'styled-components';

export interface MarkdownOptions {
  /**
   * `true` if links should be rendered. Defaults to `false` to protect from
   * rendering untrusted content posing as Teleport-approved link.
   */
  enableLinks?: boolean;
}

interface MarkdownParser {
  condition?: (opts: MarkdownOptions) => boolean;
  pattern: RegExp;
  render: (
    activeParsers: MarkdownParser[],
    content: string,
    key: string,
    url?: string
  ) => React.ReactNode;
}

interface EarliestMatch {
  match: RegExpExecArray;
  parser: MarkdownParser;
}

const StyledLink = styled.a`
  font-style: unset;
  color: ${p => p.theme.colors.buttons.link.default};
  background: none;
  text-decoration: underline;
  text-transform: none;
`;

const parsers: MarkdownParser[] = [
  {
    pattern: /\*\*(?<content>[^*]+?)\*\*/,
    render: (activeParsers, content, key) => (
      <strong key={key}>{parseLine(activeParsers, content)}</strong>
    ),
  },
  {
    pattern: /`(?<content>[^`]+)`/,
    render: (activeParsers, content, key) => <code key={key}>{content}</code>,
  },
  {
    condition: (opts: MarkdownOptions) => !!opts.enableLinks,
    pattern: /\[(?<content>[^\]]*)](?:\((?<url>https?:\/\/[^)]+|[^:)]+)\))?/,
    render: (activeParsers, content, key, url) => (
      <StyledLink key={key} href={url}>
        {parseLine(activeParsers, content)}
      </StyledLink>
    ),
  },
];

function parseLine(activeParsers: MarkdownParser[], line: string): ReactNode[] {
  const items: ReactNode[] = [];

  let remaining = line;

  let key = 0;

  while (remaining.length > 0) {
    let earliestMatch: EarliestMatch | null = null;

    for (const parser of activeParsers) {
      if (parser.pattern.test(remaining)) {
        const match = parser.pattern.exec(remaining);

        if (
          match &&
          (!earliestMatch || match.index < earliestMatch.match.index)
        ) {
          earliestMatch = { match, parser };
        }
      }
    }

    if (!earliestMatch) {
      items.push(remaining);

      break;
    }

    const { match, parser } = earliestMatch;

    if (match.index > 0) {
      items.push(remaining.substring(0, match.index));
    }

    key += 1;

    items.push(
      parser.render(
        activeParsers,
        match.groups!.content,
        `inline-${key}`,
        match.groups?.url
      )
    );

    remaining = remaining.substring(match.index + match[0].length);
  }

  return items;
}

const headerRegex = /^(?<hashes>#{1,6})\s*(?<content>.*)$/;

const MAX_ITERATIONS = 10000;

/**
 * Turns a Markdown string into a list of React nodes. CAUTION: this function
 * may handle untrusted user input, so think twice before extending it! If you
 * extend it with something that may possibly cause a security issue if
 * rendered, make sure to only enable it if a certain option is turned on and
 * default to disabling it.
 */
function processMarkdown(text: string, options: MarkdownOptions): ReactNode[] {
  if (!text) {
    return [];
  }

  const activeParsers = parsers.filter(p => {
    if (!p.condition) return true;
    return p.condition(options);
  });

  const lines = text.split('\n');

  const items: ReactNode[] = [];

  let iterations = 0;
  let i = 0;

  while (i < lines.length) {
    if (++iterations > MAX_ITERATIONS) {
      // eslint-disable-next-line no-console
      console.error(
        `processMarkdown: Exceeded max iterations (${MAX_ITERATIONS})`
      );
      return items;
    }

    const line = lines[i].trim();

    if (line.trim() === '') {
      i += 1;

      continue;
    }

    if (headerRegex.test(line)) {
      const headerMatch = headerRegex.exec(line);

      if (headerMatch) {
        const [, hashes, content] = headerMatch;
        const level = hashes.length;

        items.push(createElement(`h${level}`, { key: i }, content.trim()));

        i += 1;

        continue;
      }
    }

    if (line.trimStart().startsWith('- ')) {
      const listItems: ReactNode[] = [];
      const startI = i;

      while (i < lines.length && lines[i].trimStart().startsWith('- ')) {
        const firstDashIndex = lines[i].indexOf('- ');

        listItems.push(
          <li key={i}>
            {parseLine(
              activeParsers,
              lines[i].substring(firstDashIndex + 2).trim()
            )}
          </li>
        );

        i += 1;

        if (i - startI > MAX_ITERATIONS) {
          break;
        }
      }

      items.push(<ul key={i}>{listItems}</ul>);

      continue;
    }

    const paragraphLines: string[] = [];
    const startI = i;

    while (i < lines.length && lines[i].trim() !== '') {
      const currentLine = lines[i];

      if (
        headerRegex.test(currentLine) ||
        currentLine.trim().startsWith('- ')
      ) {
        break;
      }

      paragraphLines.push(currentLine.trim());
      i += 1;

      if (i - startI > MAX_ITERATIONS) {
        break;
      }
    }

    if (paragraphLines.length > 0) {
      items.push(
        <p key={`p-${i}`}>
          {parseLine(activeParsers, paragraphLines.join(' '))}
        </p>
      );
    }

    if (i === startI) {
      i += 1;
    }
  }

  return items;
}

export interface MarkdownProps extends MarkdownOptions {
  text: string;
}

export function Markdown({ text, ...options }: MarkdownProps) {
  return useMemo(
    () => processMarkdown(text, options),
    [text, ...Object.values(options)]
  );
}
