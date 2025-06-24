import { createElement, type ReactNode, useMemo } from 'react';
import styled from 'styled-components';

interface MarkdownParser {
  pattern: RegExp;
  render: (content: string, key: string, url?: string) => React.ReactNode;
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
    render: (content, key) => <strong key={key}>{parseLine(content)}</strong>,
  },
  {
    pattern: /`(?<content>[^`]+)`/,
    render: (content, key) => <code key={key}>{content}</code>,
  },
  {
    pattern: /\[(?<content>[^\]]*)](?:\((?<url>[^)]*)\))?/,
    render: (content, key, url) => <StyledLink key={key} href={url}>{content}</StyledLink>,
  },
];

function parseLine(line: string): ReactNode[] {
  const items: ReactNode[] = [];

  let remaining = line;

  let key = 0;

  while (remaining.length > 0) {
    let earliestMatch: EarliestMatch | null = null;

    for (const parser of parsers) {
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

    items.push(parser.render(match.groups!.content, `inline-${key}`, match.groups?.url));

    remaining = remaining.substring(match.index + match[0].length);
  }

  return items;
}

const headerRegex = /^(?<hashes>#{1,6})\s*(?<content>.*)$/;

function processMarkdown(text: string) {
  const lines = text.split('\n');

  const items: ReactNode[] = [];

  let i = 0;

  while (i < lines.length) {
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

    if (line.startsWith('- ')) {
      const listItems: ReactNode[] = [];

      while (i < lines.length && lines[i].startsWith('- ')) {
        listItems.push(<li key={i}>{parseLine(lines[i].substring(2))}</li>);

        i += 1;
      }

      items.push(<ul key={i}>{listItems}</ul>);

      continue;
    }

    const paragraphLines: string[] = [];

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
    }

    if (paragraphLines.length > 0) {
      items.push(<p key={`p-${i}`}>{parseLine(paragraphLines.join(' '))}</p>);
    }
  }

  return items;
}

interface MarkdownProps {
  text: string;
}

export function Markdown({ text }: MarkdownProps) {
  return useMemo(() => processMarkdown(text), [text]);
}
