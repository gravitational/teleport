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

const StyledPre = styled.pre`
  background: ${p => p.theme.colors.spotBackground[1]};
  border-radius: ${p => p.theme.radii[2]}px;
  padding: ${p => p.theme.space[2]}px ${p => p.theme.space[3]}px;
  overflow-x: auto;
  white-space: pre;
  margin: ${p => p.theme.space[2]}px 0;
`;

const StyledTable = styled.table`
  border-collapse: separate;
  border-spacing: 0;
  border: 1px solid ${p => p.theme.colors.spotBackground[2]};
  border-radius: ${p => p.theme.radii[2]}px;
  margin: ${p => p.theme.space[2]}px 0;
  overflow: hidden;

  th,
  td {
    border-bottom: 1px solid ${p => p.theme.colors.spotBackground[2]};
    border-right: 1px solid ${p => p.theme.colors.spotBackground[2]};
    padding: ${p => p.theme.space[1]}px ${p => p.theme.space[2]}px;
    text-align: left;
  }

  th:last-child,
  td:last-child {
    border-right: none;
  }

  tr:last-child td {
    border-bottom: none;
  }

  th {
    background: ${p => p.theme.colors.spotBackground[1]};
  }
`;

const StyledUl = styled.ul<{ root?: boolean }>`
  padding-left: ${p => p.theme.space[3]}px;
  margin-bottom: ${p => (p.root ? `${p.theme.space[3]}px` : '0')};
`;

const StyledOl = styled.ol<{ root?: boolean }>`
  padding-left: ${p => p.theme.space[3]}px;
  margin-bottom: ${p => (p.root ? `${p.theme.space[3]}px` : '0')};
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
      <StyledLink
        key={key}
        href={url}
        target="_blank"
        rel="noopener noreferrer"
      >
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

type ListType = 'ul' | 'ol';

function isListItem(trimmed: string): ListType | null {
  if (trimmed.startsWith('- ')) {
    return 'ul';
  }

  if (orderedItemRegex.test(trimmed)) {
    return 'ol';
  }

  return null;
}

function getItemContent(trimmed: string) {
  if (trimmed.startsWith('- ')) {
    return trimmed.substring(2);
  }

  const match = trimmed.match(orderedItemRegex);
  if (match) {
    return trimmed.substring(match[0].length);
  }

  return trimmed;
}

function getAlignment(sep: string): 'left' | 'center' | 'right' {
  const t = sep.trim();
  if (t.startsWith(':') && t.endsWith(':')) {
    return 'center';
  }

  if (t.endsWith(':')) {
    return 'right';
  }

  return 'left';
}

function parseRow(row: string) {
  return row
    .replace(/^\|/, '')
    .replace(/\|$/, '')
    .split(/(?<!\\)\|/)
    .map(cell => cell.trim().replace(/\\\|/g, '|'));
}

const MAX_LIST_DEPTH = 10;

function parseListItems(
  activeParsers: MarkdownParser[],
  lines: string[],
  startIndex: number,
  baseIndent: number,
  listType: ListType,
  isRoot = false,
  depth = 0
): { node: ReactNode; endIndex: number } {
  if (depth >= MAX_LIST_DEPTH) {
    return { node: null, endIndex: startIndex };
  }

  const listItems: ReactNode[] = [];

  let startNumber: number | undefined;
  let i = startIndex;

  while (i < lines.length) {
    const raw = lines[i];
    const trimmed = raw.trimStart();

    const itemType = isListItem(trimmed);
    if (!itemType || itemType !== listType) {
      break;
    }

    const indent = raw.length - trimmed.length;
    if (indent < baseIndent) {
      break;
    }

    // Capture the start number from the first ordered list item.
    if (listType === 'ol' && startNumber === undefined) {
      const num = parseInt(trimmed, 10);
      if (num !== 1) {
        startNumber = num;
      }
    }

    const content = getItemContent(trimmed);
    const itemKey = i;

    i += 1;

    const contentParts = [content];
    while (i < lines.length) {
      const next = lines[i];
      const nextTrimmed = next.trimStart();
      const nextIndent = next.length - nextTrimmed.length;

      if (
        nextTrimmed === '' ||
        isListItem(nextTrimmed) ||
        nextIndent <= baseIndent
      ) {
        // If the next line is blank, a new list item, or less indented than the base,
        // it's not part of the current item's content.
        break;
      }

      contentParts.push(nextTrimmed);
      i += 1;
    }

    const fullContent = contentParts.join(' ');

    // Skip blank lines before checking for nested items.
    let nestedList: ReactNode = null;
    let blankSkip = 0;

    while (i + blankSkip < lines.length && lines[i + blankSkip].trim() === '') {
      blankSkip += 1;
    }

    // Check for nested list items after the current item, allowing for blank lines in between.
    if (i + blankSkip < lines.length) {
      const nextRaw = lines[i + blankSkip];
      const nextTrimmed = nextRaw.trimStart();
      const nextIndent = nextRaw.length - nextTrimmed.length;
      const nextType = isListItem(nextTrimmed);

      if (nextType && nextIndent > baseIndent) {
        // If we find a nested list item, parse it and attach it to the current item.
        i += blankSkip;

        const nested = parseListItems(
          activeParsers,
          lines,
          i,
          nextIndent,
          nextType,
          false,
          depth + 1
        );

        nestedList = nested.node;
        i = nested.endIndex;
      }
    }

    listItems.push(
      <li key={itemKey}>
        {parseLine(activeParsers, fullContent)}
        {nestedList}
      </li>
    );

    if (i - startIndex > MAX_ITERATIONS) {
      break;
    }
  }

  const key = `list-${startIndex}`;
  const node =
    listType === 'ol' ? (
      <StyledOl key={key} root={isRoot} start={startNumber}>
        {listItems}
      </StyledOl>
    ) : (
      <StyledUl key={key} root={isRoot}>
        {listItems}
      </StyledUl>
    );

  return { node, endIndex: i };
}

const headerRegex = /^(?<hashes>#{1,6})\s*(?<content>.*)$/;
const fencedCodeRegex = /^```(\w*)\s*$/;
const orderedItemRegex = /^\d+\.\s/;
const tableRowRegex = /^\|([^|]*\|){2,}$/;
const tableSeparatorRegex = /^\|([\s:]*-+[\s:]*\|)+$/;

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

        items.push(
          createElement(
            `h${level}`,
            { key: i },
            ...parseLine(activeParsers, content.trim())
          )
        );

        i += 1;

        continue;
      }
    }

    const listType = isListItem(line);
    if (listType) {
      const baseIndent = lines[i].length - lines[i].trimStart().length;
      const result = parseListItems(
        activeParsers,
        lines,
        i,
        baseIndent,
        listType,
        true
      );

      items.push(result.node);
      i = result.endIndex;

      continue;
    }

    // Fenced code blocks: ```lang ... ```
    if (fencedCodeRegex.test(line)) {
      const codeLines: string[] = [];
      const startI = i;
      i += 1; // skip the opening fence

      while (i < lines.length && !fencedCodeRegex.test(lines[i].trim())) {
        codeLines.push(lines[i]);
        i += 1;

        if (i - startI > MAX_ITERATIONS) {
          break;
        }
      }

      // Skip the closing fence if we found one.
      if (i < lines.length) {
        i += 1;
      }

      items.push(
        <StyledPre key={`code-${i}`}>
          <code>{codeLines.join('\n')}</code>
        </StyledPre>
      );

      continue;
    }

    // Tables: | col1 | col2 |
    if (tableRowRegex.test(line)) {
      const tableLines: string[] = [];
      const startI = i;

      while (i < lines.length && tableRowRegex.test(lines[i].trim())) {
        tableLines.push(lines[i].trim());
        i += 1;

        if (i - startI > MAX_ITERATIONS) {
          break;
        }
      }

      // We need at least 2 lines for a valid table (header + separator)
      if (tableLines.length >= 2) {
        const hasHeader = tableSeparatorRegex.test(tableLines[1]);
        const headerCells = hasHeader ? parseRow(tableLines[0]) : [];
        const alignments = hasHeader
          ? tableLines[1]
              .replace(/^\|/, '')
              .replace(/\|$/, '')
              .split(/(?<!\\)\|/)
              .map(getAlignment)
          : [];
        const dataRows = tableLines
          .slice(hasHeader ? 2 : 0)
          .map(row => parseRow(row));

        items.push(
          <StyledTable key={`table-${startI}`}>
            {hasHeader && (
              <thead>
                <tr>
                  {headerCells.map((cell, ci) => (
                    <th
                      key={ci}
                      style={
                        alignments[ci] && alignments[ci] !== 'left'
                          ? { textAlign: alignments[ci] }
                          : undefined
                      }
                    >
                      {parseLine(activeParsers, cell)}
                    </th>
                  ))}
                </tr>
              </thead>
            )}
            <tbody>
              {dataRows.map((row, ri) => (
                <tr key={ri}>
                  {row.map((cell, ci) => (
                    <td
                      key={ci}
                      style={
                        alignments[ci] && alignments[ci] !== 'left'
                          ? { textAlign: alignments[ci] }
                          : undefined
                      }
                    >
                      {parseLine(activeParsers, cell)}
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </StyledTable>
        );
      } else {
        // Single line starting with |, treat as paragraph.
        items.push(
          <p key={`p-${startI}`}>{parseLine(activeParsers, tableLines[0])}</p>
        );
      }

      continue;
    }

    const paragraphLines: string[] = [];
    const startI = i;

    while (i < lines.length && lines[i].trim() !== '') {
      const currentLine = lines[i];

      if (
        headerRegex.test(currentLine) ||
        isListItem(currentLine.trim()) ||
        fencedCodeRegex.test(currentLine.trim()) ||
        tableRowRegex.test(currentLine.trim())
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
