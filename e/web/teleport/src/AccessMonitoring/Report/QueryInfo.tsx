import Highlight from 'react-highlight';
import { Link } from 'react-router-dom';
import styled from 'styled-components';

import { ChevronRight, Code } from 'design/Icon';

import { PopoverHeader } from 'e-teleport/AccessMonitoring/shared/Popover';
import cfg from 'e-teleport/config';

interface QueryInfoProps {
  query: string;
  days: number;
}

const Container = styled.div`
  display: flex;
  flex-direction: column;
`;

const Query = styled.div`
  font-family: ${p => p.theme.fonts.mono};
  font-size: 12px;
  padding: 0 12px;

  pre {
    margin: 0;
  }

  code {
    padding: 0;
    line-height: 1.4;
  }

  pre code.hljs {
    display: block;
    overflow-x: auto;
    padding: 0;
  }

  code.hljs {
    padding: 3px 5px;
  }

  .hljs {
    color: ${p => p.theme.colors.text.main};
  }

  .hljs-doctag,
  .hljs-keyword,
  .hljs-meta .hljs-keyword,
  .hljs-template-tag,
  .hljs-template-variable,
  .hljs-type,
  .hljs-variable.language_ {
    color: ${p => p.theme.colors.editor.abbey};
  }

  .hljs-attr,
  .hljs-attribute,
  .hljs-literal,
  .hljs-meta,
  .hljs-number,
  .hljs-operator,
  .hljs-selector-attr,
  .hljs-selector-class,
  .hljs-selector-id,
  .hljs-variable {
    color: ${p => p.theme.colors.editor.cyan};
  }

  .hljs-meta .hljs-string,
  .hljs-regexp,
  .hljs-string {
    color: ${p => p.theme.colors.editor.cyan};
  }

  .hljs-built_in,
  .hljs-symbol {
    color: ${p => p.theme.colors.editor.purple};
  }
`;

const OpenInLink = styled(Link)`
  font-family: ${p => p.theme.fonts.mono};
  text-transform: uppercase;
  line-height: 1;
  font-size: 12px;
  border-radius: 7px;
  color: ${p => p.theme.colors.text.main};
  cursor: pointer;
  padding: 4px 4px 4px 12px;
  display: flex;
  align-items: center;
  user-select: none;
  gap: 4px;
  text-decoration: none;

  &:hover {
    background: ${p => p.theme.colors.spotBackground[0]};
  }
`;

const Footer = styled.div`
  display: flex;
  justify-content: flex-end;
  padding: 0 4px 4px;
`;

export function QueryInfo(props: QueryInfoProps) {
  return (
    <Container>
      <PopoverHeader>
        <Code size="small" />
        Query
      </PopoverHeader>

      <Query>
        <Highlight className="sql">{props.query}</Highlight>
      </Query>

      <Footer>
        <OpenInLink
          to={{
            pathname: cfg.routes.accessMonitoring.queryEditor,
            state: { query: props.query.trim(), days: props.days },
          }}
        >
          Open in Query Editor <ChevronRight size={18} />
        </OpenInLink>
      </Footer>
    </Container>
  );
}
