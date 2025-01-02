import { Completion } from '@codemirror/autocomplete';
import { sql, SQLDialect } from '@codemirror/lang-sql';
import { EditorView } from '@codemirror/view';
import { tags as t } from '@lezer/highlight';
import { createTheme } from '@uiw/codemirror-themes';
import CodeMirror from '@uiw/react-codemirror';
import React, { useEffect, useMemo, useState } from 'react';
import { useLocation } from 'react-router';
import styled, { useTheme } from 'styled-components';

import { Button, Flex } from 'design';
import Indicator from 'design/Indicator';
import { darkTheme, lightTheme } from 'design/theme';
import { Theme } from 'design/theme/themes/types';
import { useAttemptNext } from 'shared/hooks';

import { Result } from 'e-teleport/AccessMonitoring/QueryEditor/Result';
import { getSchema, runQuery } from 'e-teleport/AccessMonitoring/service';
import { Days, Timeframe } from 'e-teleport/AccessMonitoring/Timeframe';
import useStickyClusterId from 'teleport/useStickyClusterId';

import column from './icons/column.svg';
import cube from './icons/cube.svg';
import keyword from './icons/keyword.svg';
import object from './icons/object.svg';
import table from './icons/table.svg';
import variable from './icons/variable.svg';

const CustomDialect = SQLDialect.define({
  keywords:
    'and as or by group order interval distinct contains count from where select',
  types: '',
});

const Query = styled.div<{ disabled?: boolean }>`
  background: ${p => p.theme.colors.levels.popout};
  border: 1px solid ${p => p.theme.colors.spotBackground[1]};
  padding: ${p => p.theme.space[3]}px ${p => p.theme.space[2]}px
    ${p => p.theme.space[3]}px ${p => p.theme.space[2]}px;
  margin-top: -1px;
  border-radius: 7px;
  position: relative;
  display: flex;
  opacity: ${p => (p.disabled ? 0.5 : 1)};
  pointer-events: ${p => (p.disabled ? 'none' : 'auto')};
`;

const Sidebar = styled.div`
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  padding-right: ${p => p.theme.space[2]}px;
`;

const ExecuteContainer = styled.div`
  display: flex;
  align-items: center;
  gap: ${p => p.theme.space[3]}px;
`;

const Key = styled.div`
  line-height: 1;
  background: ${p => p.theme.colors.spotBackground[1]};
  padding: 2px;
  border: 1px solid ${p => p.theme.colors.spotBackground[1]};
  border-radius: 4px;
`;

const KeyShortcut = styled.div`
  display: flex;
  align-items: center;
  gap: ${p => p.theme.space[1]}px;
  color: ${p => p.theme.colors.text.muted};
  font-size: 12px;
  pointer-events: none;
  user-select: none;
`;

const EditorContainer = styled.div`
  flex: 1;
  padding-bottom: 28px;
`;

type SchemaObject = Record<string, Completion[]>;

interface CodeMirrorConfig {
  schema: SchemaObject;
  tables: Completion[];
}

export function QueryEditor() {
  const theme = useTheme();
  const location = useLocation<{ query: string; days: number }>();

  const { clusterId } = useStickyClusterId();

  const schema = useAttemptNext('processing');
  const query = useAttemptNext('');

  const [days, setDays] = useState<Days>((location.state?.days as Days) || 7);
  const [queryText, setQueryText] = useState<string>(
    location.state?.query || ''
  );
  const [resultId, setResultId] = useState<string | null>(null);

  const editorTheme = useMemo(() => createEditorTheme(theme), [theme]);
  const styleTheme = useMemo(() => createStyleTheme(theme), [theme]);

  const [config, setConfig] = useState<CodeMirrorConfig>({
    schema: {},
    tables: [],
  });

  useEffect(() => {
    async function init() {
      const schema = await getSchema(clusterId);

      const schemaObj: SchemaObject = {};

      for (const table of schema) {
        schemaObj[table.name.replace('.', '_')] = table.columns.map(col => ({
          label: col.name,
          type: 'column',
        }));
      }

      const tables = schema.map(table => ({
        label: table.name.replace('.', '_'),
        type: 'table',
      }));

      setConfig({
        schema: schemaObj,
        tables,
      });
    }

    schema.run(init);
  }, [clusterId]);

  function handleChange(value: string) {
    setQueryText(value);
  }

  function handleRunQuery() {
    setResultId(null);

    query.run(async function () {
      const res = await runQuery(clusterId, queryText, days);

      setResultId(res.result_id);
    });
  }

  function handleKeyDown(e: React.KeyboardEvent) {
    if (e.key === 'Enter' && e.metaKey) {
      e.preventDefault();
      e.stopPropagation();

      handleRunQuery();
    }
  }

  if (schema.attempt.status === 'processing') {
    return (
      <Flex alignItems="center" justifyContent="center" m={4}>
        <Indicator />
      </Flex>
    );
  }

  // if the attempt has failed, only the schema for autocomplete is missing
  // this means we can still render the editor

  return (
    <>
      <Query disabled={query.attempt.status === 'processing'}>
        <EditorContainer>
          <CodeMirror
            value={queryText}
            minHeight="100px"
            onKeyDownCapture={handleKeyDown}
            extensions={[
              sql({
                ...config,
                dialect: CustomDialect,
                schemas: [],
                upperCaseKeywords: true,
              }),
              EditorView.lineWrapping,
              styleTheme,
            ]}
            autoFocus={true}
            placeholder="Write your query here..."
            onChange={handleChange}
            theme={editorTheme}
            basicSetup={{
              lineNumbers: false,
              defaultKeymap: false,
              highlightActiveLine: false,
              highlightActiveLineGutter: false,
              indentOnInput: false,
            }}
          />
        </EditorContainer>

        <Sidebar>
          <Timeframe days={days} onChange={setDays} />

          <ExecuteContainer>
            <KeyShortcut>
              <Key>⌘</Key>+<Key>Enter</Key>
            </KeyShortcut>

            <Button onClick={handleRunQuery} disabled={!queryText}>
              Run Query
            </Button>
          </ExecuteContainer>
        </Sidebar>
      </Query>

      {query.attempt.status === 'processing' && (
        <Flex alignItems="center" justifyContent="center" m={4}>
          <Indicator />
        </Flex>
      )}

      {resultId && <Result resultId={resultId} />}

      {query.attempt.status === 'failed' && (
        <Flex alignItems="center" justifyContent="center" m={4}>
          An error occurred running the query: {query.attempt.statusText}
        </Flex>
      )}
    </>
  );
}

function createEditorTheme(theme: typeof lightTheme | typeof darkTheme) {
  return createTheme({
    theme: theme.type === 'light' ? 'light' : 'dark',
    settings: {
      background: theme.colors.levels.popout,
      foreground: theme.colors.text.main,
      caret: theme.colors.terminal.cursor,
      selection: theme.colors.spotBackground[0],
      selectionMatch: theme.colors.spotBackground[1],
      lineHighlight: theme.colors.levels.popout,
      gutterBackground: theme.colors.levels.popout,
      gutterForeground: theme.colors.text.muted,
      fontFamily: theme.fonts.mono,
      gutterBorder: theme.colors.levels.popout,
    },
    styles: [
      { tag: t.comment, color: theme.colors.text.muted },
      { tag: t.variableName, color: theme.colors.editor.caribbean },
      {
        tag: [t.string, t.special(t.brace)],
        color: theme.colors.editor.sunflower,
      },
      { tag: t.number, color: theme.colors.editor.sunflower },
      { tag: t.bool, color: theme.colors.editor.picton },
      { tag: t.null, color: theme.colors.editor.abbey },
      { tag: t.keyword, color: theme.colors.editor.abbey },
      { tag: t.operator, color: theme.colors.text.main },
      { tag: t.className, color: theme.colors.text.main },
      { tag: t.definition(t.typeName), color: theme.colors.text.main },
      { tag: t.typeName, color: theme.colors.text.main },
      { tag: t.angleBracket, color: theme.colors.text.main },
      { tag: t.tagName, color: theme.colors.text.main },
      { tag: t.attributeName, color: theme.colors.text.main },
    ],
  });
}

function createStyleTheme(theme: Theme) {
  return EditorView.theme({
    '&.cm-editor.cm-focused': {
      outline: 'none',
    },
    '.cm-lineNumbers': {
      width: '35px',
    },
    '.cm-tooltip.cm-tooltip-autocomplete': {
      border: `1px solid rgba(0, 0, 0, 0.3)`,
      background: theme.colors.levels.popout,
      boxShadow: '0 3px 4px rgba(0, 0, 0, 0.2)',
      borderRadius: '14px',
      overflow: 'hidden',
      fontFamily: `${theme.fonts.mono} !important`,
    },
    '.cm-tooltip-autocomplete ul': {
      padding: `${theme.space[2]}px !important`,
    },
    '.cm-tooltip-autocomplete ul li': {
      display: 'flex',
      alignItems: 'center',
      padding: `${theme.space[2]}px !important`,
      borderRadius: '10px',
      gap: `${theme.space[2]}px`,
      position: 'relative',
    },
    '.cm-tooltip.cm-tooltip-autocomplete > ul > li[aria-selected]': {
      background: theme.colors.spotBackground[0],
      color: theme.colors.text.main,
      fontSize: '14px',
    },
    '.cm-completionIcon-type:after': {
      display: 'block',
      background: `url(${cube})`,
      backgroundSize: 'contain',
      width: '16px',
      height: '16px',
      opacity: '0.8',
      content: `''`,
    },
    '.cm-completionIcon-object:after': {
      display: 'block',
      background: `url(${object})`,
      backgroundSize: 'contain',
      width: '16px',
      height: '16px',
      opacity: '0.8',
      content: `''`,
    },
    '.cm-completionIcon-variable:after': {
      display: 'block',
      background: `url(${variable})`,
      backgroundSize: 'contain',
      width: '16px',
      height: '16px',
      opacity: '0.8',
      content: `''`,
    },
    '.cm-completionIcon-property:after': {
      display: 'block',
      background: `url(${cube})`,
      backgroundSize: 'contain',
      width: '16px',
      height: '16px',
      opacity: '0.8',
      content: `''`,
    },
    '.cm-completionIcon-boolean:after': {
      display: 'block',
      background: `url(${cube})`,
      backgroundSize: 'contain',
      width: '16px',
      height: '16px',
      opacity: '0.8',
      content: `''`,
    },
    '.cm-placeholder': {
      color: theme.colors.text.muted,
    },
    '.cm-completionIcon-table:after': {
      display: 'block',
      background: `url(${table})`,
      backgroundSize: 'contain',
      width: '16px',
      height: '16px',
      opacity: '0.8',
      content: `''`,
    },
    '.cm-completionIcon-column:after': {
      display: 'block',
      background: `url(${column})`,
      backgroundSize: 'contain',
      width: '16px',
      height: '16px',
      opacity: '0.8',
      content: `''`,
    },
    '.cm-completionLabel': {
      paddingRight: '86px',
    },
    '.cm-completionIcon-variable + .cm-completionLabel::after': {
      content: `'variable'`,
      position: 'absolute',
      right: '16px',
      opacity: '0.4',
      fontWeight: '500',
      fontSize: '12px',
      top: '50%',
      transform: 'translateY(-50%)',
    },
    '.cm-completionIcon-boolean + .cm-completionLabel::after': {
      content: `'boolean'`,
      position: 'absolute',
      right: '16px',
      opacity: '0.4',
      fontWeight: '500',
      fontSize: '12px',
      top: '50%',
      transform: 'translateY(-50%)',
    },
    '.cm-completionIcon-object + .cm-completionLabel::after': {
      content: `'object'`,
      position: 'absolute',
      right: '16px',
      opacity: '0.4',
      fontWeight: '500',
      fontSize: '12px',
      top: '50%',
      transform: 'translateY(-50%)',
    },
    '.cm-completionIcon-property + .cm-completionLabel::after': {
      content: `'property'`,
      position: 'absolute',
      right: '16px',
      opacity: '0.4',
      fontWeight: '500',
      fontSize: '12px',
      top: '50%',
      transform: 'translateY(-50%)',
    },
    '.cm-completionIcon-keyword + .cm-completionLabel::after': {
      content: `'keyword'`,
      position: 'absolute',
      right: '16px',
      opacity: '0.4',
      fontWeight: '500',
      fontSize: '12px',
      top: '50%',
      transform: 'translateY(-50%)',
    },
    '.cm-completionIcon-keyword::before': {
      background: `url(${keyword}) no-repeat center center`,
      backgroundSize: '16px',
    },
  });
}
