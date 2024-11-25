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

import React, { useState } from 'react';
import styled, { keyframes } from 'styled-components';

import {
  KeywordHighlight,
  TerminalColor,
} from 'shared/components/AnimatedTerminal/TerminalContent';
import { Language } from 'shared/components/Editor/Language';
import { Tabs } from 'shared/components/Editor/Tabs';
import {
  WindowCode,
  WindowContainer,
  WindowContentContainer,
  WindowTitleBar,
} from 'shared/components/Window';

import { File, FileProps } from './File';

interface EditorProps {
  title: string;
}

export function Editor(props: React.PropsWithChildren<EditorProps>) {
  const [activeTabIndex, setActiveTabIndex] = useState(0);

  const files = React.Children.map(
    props.children,
    (child: React.ReactElement<FileProps>) => {
      if (child.type === File) {
        return {
          name: child.props.name,
          content: child.props.code,
          language: child.props.language,
        };
      }

      return null;
    }
  ).filter(Boolean);

  const tabs = files.map(file => file.name);

  const { content, language } = files[activeTabIndex];

  const parsed = parse(content, language);

  const lineNumbers = [];
  if (content) {
    const numberOfLines = content.split('\n').length;

    for (let i = 0; i <= numberOfLines; i++) {
      lineNumbers.push(
        <LineNumber
          key={i}
          data-line-number={i + 1}
          active={i === numberOfLines}
        />
      );
    }
  } else {
    lineNumbers.push(<LineNumber key={0} data-line-number={1} active />);
  }

  return (
    <WindowContainer>
      <WindowTitleBar title={props.title} />

      <Tabs
        items={tabs}
        activeIndex={activeTabIndex}
        onSelect={setActiveTabIndex}
      />

      <WindowContentContainer style={{ height: 585 }}>
        <WindowCode style={{ display: 'flex' }}>
          <LineNumbers>{lineNumbers}</LineNumbers>
          <Code>
            {parsed}
            <ActiveLine>
              <Cursor />
            </ActiveLine>
          </Code>
        </WindowCode>
      </WindowContentContainer>
    </WindowContainer>
  );
}

function parse(code: string, language: Language) {
  switch (language) {
    case Language.YAML:
      return parseYAML(code);
    default:
      throw new Error('Language not supported');
  }
}

function parseYAML(code: string) {
  if (!code) {
    return [];
  }

  const highlights = [
    {
      key: 'string',
      keywords: [`'\\*'`],
      color: TerminalColor.Keyword,
    },
    {
      key: 'certificate',
      match: /(-----.*?-----)/,
      color: TerminalColor.Punctuation,
    },
  ];

  const lines = code.split('\n');

  const result = [];
  for (const [index, line] of lines.entries()) {
    const highlightSemicolonMultiline = highlight(
      line,
      ': |',
      index,
      highlights
    );
    if (highlightSemicolonMultiline) {
      result.push(highlightSemicolonMultiline);

      continue;
    }

    const highlightSemicolon = highlight(line, ':', index, highlights);
    if (highlightSemicolon) {
      result.push(highlightSemicolon);

      continue;
    }

    if (line) {
      result.push(<div key={index}>{highlightValue(line, highlights)}</div>);

      continue;
    }

    result.push(<div key={index}>&nbsp;</div>);
  }

  return result;
}

function highlight(
  code: string,
  symbol: string,
  index: number,
  highlights: KeywordHighlight[]
) {
  if (!code.includes(symbol)) {
    return;
  }

  const symbolIndex = code.indexOf(symbol);

  let value = code.substring(symbolIndex + symbol.length, code.length);

  return (
    <div key={index}>
      <Keyword>{code.substring(0, symbolIndex)}</Keyword>
      <Punctuation>{symbol}</Punctuation>
      {highlightValue(value, highlights)}
    </div>
  );
}

function highlightValue(content: string, highlights: KeywordHighlight[]) {
  for (const entry of highlights) {
    if (entry.match && entry.match.test(content)) {
      const split = content.split(entry.match);

      return split
        .map((item, index) => {
          if (!item) {
            return;
          }

          // all odd occurrences are matches, the rest remain unchanged
          if (index % 2 === 0) {
            return <span key={index}>{item}</span>;
          }

          return (
            <span key={`${entry.key}-${index}`} style={{ color: entry.color }}>
              {item}
            </span>
          );
        })
        .filter(Boolean);
    }
  }

  const words = content.split(' ');
  const result = [];

  outer: for (const [index, word] of words.entries()) {
    for (const entry of highlights) {
      if (entry.keywords) {
        const highlightedWords = highlightWord(word, entry);
        if (highlightedWords) {
          result.push(
            <span key={`${entry.key}-${index}`}>{highlightedWords} </span>
          );

          continue outer;
        }
      }
    }

    result.push(<span key={index}>{word} </span>);
  }

  return result;
}

function highlightWord(content: string, highlight: KeywordHighlight) {
  const regex = new RegExp(`(${highlight.keywords.join('|')})`);

  if (regex.test(content)) {
    const split = content.split(regex);

    return split
      .map((item, index) => {
        if (!item) {
          return;
        }

        // all odd occurrences are matches, the rest remain unchanged
        if (index % 2 === 0) {
          return <span key={index}>{item}</span>;
        }

        return (
          <span
            key={`${highlight.key}-${index}`}
            style={{ color: highlight.color }}
          >
            {item}
          </span>
        );
      })
      .filter(Boolean);
  }

  return null;
}

const Keyword = styled.span`
  color: #d4656b;
`;

const Punctuation = styled.span`
  color: #81ceee;
`;

const blink = keyframes`
  0% {
    opacity: 0;
  }
`;

const Cursor = styled.span`
  display: inline-block;
  width: 2px;
  height: 15px;
  background: #ffffff;
  vertical-align: middle;
  animation: ${blink} 1.5s steps(2) infinite;
`;

const LineNumbers = styled.div`
  user-select: none;
  width: 55px;
`;

interface LineNumberProps {
  active?: boolean;
}

const LineNumber = styled.div<LineNumberProps>`
  background: ${p => (p.active ? 'rgba(0, 0, 0, 0.3)' : 'none')};
  color: ${p =>
    p.active ? 'rgba(255, 255, 255, 0.6)' : 'rgba(255, 255, 255, 0.3)'};
  text-align: right;
  padding-right: 20px;

  &:before {
    content: attr(data-line-number);
  }
`;

const Code = styled.div`
  width: 100%;
  color: white;
`;

const ActiveLine = styled.div`
  background: rgba(0, 0, 0, 0.3);
  width: 100%;
`;
