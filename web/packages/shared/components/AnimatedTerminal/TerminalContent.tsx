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

import React, { useEffect, useLayoutEffect, useRef } from 'react';
import styled from 'styled-components';

import { BufferEntry } from 'shared/components/AnimatedTerminal/content';

export interface SelectedLines {
  start: number;
  end: number;
}

interface TerminalContentProps {
  lines: BufferEntry[];
  completed: boolean;
  counter: number;
  selectedLines?: SelectedLines;
  highlights?: KeywordHighlight[];
}

export interface KeywordHighlight {
  key: string;
  match?: RegExp;
  keywords?: string[];
  color: TerminalColor;
}

export enum TerminalColor {
  Argument = '#cfa7ff',
  Keyword = '#5af78e',
  Error = '#f07278',
  Label = 'rgba(255, 255, 255, 0.7)',
  Punctuation = '#81ceee',
}

const SelectedLinesOverlay = styled.div`
  width: 100%;
  background: rgba(255, 255, 255, 0.3);
  position: absolute;
  left: 0;
  z-index: 0;
  transform: translate3d(0, 0, 0);
  transition-property: height;
`;

const Lines = styled.div`
  position: relative;
  z-index: 1;
`;

export function TerminalContent(props: TerminalContentProps) {
  const ref = useRef<HTMLDivElement>();

  useLayoutEffect(() => {
    ref.current.scrollTop = ref.current.scrollHeight;
  }, [props.counter]);

  const renderedLines = useRef<HTMLDivElement>();

  useEffect(() => {
    if (!props.selectedLines) {
      return;
    }

    const numberOfLines = props.selectedLines.end - props.selectedLines.start;

    const id = window.setTimeout(() => {
      renderedLines.current.style.height = `${20 * (numberOfLines + 1)}px`;
    }, 1000);

    return () => clearTimeout(id);
  }, [props.selectedLines]);

  let selectedLines;
  if (props.selectedLines) {
    const numberOfLines = props.selectedLines.end - props.selectedLines.start;

    selectedLines = (
      <SelectedLinesOverlay
        ref={renderedLines}
        style={{
          top: 20 * (props.selectedLines.start + 1),
          transitionTimingFunction: `steps(${numberOfLines + 2}, jump-none)`,
          transitionDuration: `${numberOfLines * 0.08}s`,
          height: 0,
        }}
      />
    );
  }

  return (
    <TerminalContentContainer ref={ref}>
      <TerminalCode>
        <Lines>{renderLines(props.lines, props.highlights)}</Lines>

        {selectedLines}
      </TerminalCode>
    </TerminalContentContainer>
  );
}

function renderLines(lines: BufferEntry[], highlights?: KeywordHighlight[]) {
  if (!lines.length) {
    return (
      <Prompt key="cursor">
        $ <Cursor />
      </Prompt>
    );
  }

  const result = lines.map(line => (
    <React.Fragment key={line.id}>
      {line.isCommand ? (
        <Prompt>${line.text.length > 0 ? ' ' : ''}</Prompt>
      ) : null}
      {formatText(line.text, line.isCommand, highlights)}
      {line.isCurrent && line.isCommand ? <Cursor /> : null}
      <br />
    </React.Fragment>
  ));

  return result;
}

function highlightWords(content: string, highlight: KeywordHighlight) {
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

function formatText(
  source: string,
  isCommand: boolean,
  highlights?: KeywordHighlight[]
) {
  let text = source;
  let comment;

  const commentStartIndex = text.indexOf('#');
  if (commentStartIndex > -1) {
    text = source.substring(0, commentStartIndex);

    comment = (
      <Comment>{source.substring(commentStartIndex, source.length)}</Comment>
    );
  }

  const words = text.split(' ');
  const result = [];

  outer: for (const [index, word] of words.entries()) {
    if (!isCommand && /(https?:\/\/\S+)/g.test(word)) {
      result.push(
        <React.Fragment key={index}>
          <a
            key={index}
            style={{ color: '#feaa01', textDecoration: 'underline' }}
            href={word}
            target="_blank"
            rel="noopener noreferrer"
          >
            {word}
          </a>{' '}
        </React.Fragment>
      );

      continue;
    }

    if (highlights) {
      for (const entry of highlights) {
        const highlightedWords = highlightWords(word, entry);
        if (highlightedWords) {
          result.push(
            <Word key={`${entry.key}-${index}`}>{highlightedWords} </Word>
          );

          continue outer;
        }
      }
    }

    result.push(<Word key={index}>{word} </Word>);
  }

  return (
    <>
      {result}
      {comment}
    </>
  );
}

const Word = styled.span`
  user-select: none;
  color: ${props => props.theme.colors.light};
`;

const Prompt = styled.span`
  user-select: none;
  color: rgb(204, 204, 204);
`;

const Comment = styled.span`
  user-select: none;
  color: rgb(255, 255, 255, 0.4);
`;

const Cursor = styled.span`
  display: inline-block;
  width: 6px;
  height: 15px;
  background: #ffffff;
  vertical-align: middle;
`;

export const TerminalContentContainer = styled.div`
  background: #04162c;
  height: inherit;
  overflow-y: auto;
  border-bottom-left-radius: 5px;
  border-bottom-right-radius: 5px;
`;

export const TerminalCode = styled.div`
  font-size: 12px;
  font-family: Menlo, DejaVu Sans Mono, Consolas, Lucida Console, monospace;
  line-height: 20px;
  white-space: pre-wrap;
  margin: 10px 16px;
  position: relative;
`;
