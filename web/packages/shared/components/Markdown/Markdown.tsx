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

import {
  createElement,
  PropsWithChildren,
  useId,
  useMemo,
  useState,
  type ReactNode,
} from 'react';
import styled from 'styled-components';

import Flex from 'design/Flex';
import { ChevronDown, ChevronRight } from 'design/Icon';
import { P2 } from 'design/Text';

import { CopyButton } from '../CopyButton/CopyButton';

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

const headerRegex = /^(?<hashes>#{1,6})\s*(?<content>.*)$/;
const fencedCodeRegex = /^```(\w*)\s*$/;

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

      items.push(<CodeBlock key={`code-${i}`} code={codeLines.join('\n')} />);

      continue;
    }

    if (line.trim().startsWith('<details')) {
      const expanded = line.includes('open');
      let summary = '';
      const content: string[] = [];
      const startI = i;
      i += 1; // skip the opening tag

      const nextLine = lines.at(i);
      if (nextLine === undefined) {
        break;
      }

      if (nextLine.trim().startsWith('<summary>')) {
        summary = nextLine.replaceAll(/<\/?summary>/g, '');
        i += 1;
      }

      while (true) {
        const nextLine = lines.at(i);
        if (nextLine === undefined) {
          break;
        }

        if (i - startI > MAX_ITERATIONS) {
          break;
        }

        // Simple check for section end prevents nested sections
        if (nextLine.trim() == '</details>') {
          i += 1;
          break;
        }

        content.push(nextLine);
        i += 1;
      }

      items.push(
        <Section
          key={`section-${i}`}
          title={summary || 'Expand'}
          expanded={expanded}
        >
          <Markdown text={content.join('\n')} {...options} />
        </Section>
      );

      continue;
    }

    const paragraphLines: string[] = [];
    const startI = i;

    while (i < lines.length && lines[i].trim() !== '') {
      const currentLine = lines[i];

      if (
        headerRegex.test(currentLine.trim()) ||
        currentLine.trim().startsWith('- ') ||
        fencedCodeRegex.test(currentLine.trim()) ||
        currentLine.trim().startsWith('<details')
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

function CodeBlock(props: { code: string }) {
  const { code } = props;
  return (
    <CodeBlockContainer>
      <StyledPre>
        <code>{code}</code>
      </StyledPre>
      <StyledCopyButton value={code} />
    </CodeBlockContainer>
  );
}

const CodeBlockContainer = styled.div`
  position: relative;
  background: ${p => p.theme.colors.interactive.tonal.neutral[1]};
  border-radius: ${p => p.theme.radii[2]}px;
  margin: ${p => p.theme.space[2]}px 0;
`;

const StyledPre = styled.pre`
  overflow-x: auto;
  white-space: pre;
  padding: ${p => p.theme.space[2]}px ${p => p.theme.space[3]}px;
  margin: 0;
`;

const StyledCopyButton = styled(CopyButton)`
  position: absolute;
  right: 0;
  top: 0;
  padding: ${({ theme }) => theme.space[2]}px;
`;

function Section(
  props: { title: string; expanded: boolean } & PropsWithChildren
) {
  const { children, title, expanded: preExpanded } = props;
  const [expanded, setExpanded] = useState(preExpanded);
  const accessibilityId = useId();

  return (
    <SectionContainer>
      <SectionHeadingContainer
        onClick={() => setExpanded(prev => !prev)}
        role="button"
        aria-expanded={expanded}
        aria-controls={accessibilityId}
        tabIndex={0}
        onKeyDown={e => {
          if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault();
            setExpanded(prev => !prev);
          }
        }}
      >
        {expanded ? (
          <ChevronDown size="medium" />
        ) : (
          <ChevronRight size="medium" />
        )}
        <P2>
          <strong>{title}</strong>
        </P2>
      </SectionHeadingContainer>
      <SectionContentContainer id={accessibilityId} hidden={!expanded}>
        {children}
      </SectionContentContainer>
    </SectionContainer>
  );
}

const SectionContainer = styled.div`
  padding: ${({ theme }) => theme.space[2]}px 0;
`;

const SectionHeadingContainer = styled(Flex)`
  cursor: pointer;
  gap: ${({ theme }) => theme.space[2]}px;
`;

const SectionContentContainer = styled.div`
  padding-left: ${({ theme }) => theme.space[3]}px;
  border-left: 4px solid
    ${({ theme }) => theme.colors.interactive.tonal.neutral[1]};
`;
