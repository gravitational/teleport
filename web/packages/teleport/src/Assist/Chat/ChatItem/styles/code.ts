import { css } from 'styled-components';

export const codeCSS = css`
  pre .hljs-comment,
  pre .hljs-title {
    color: #7285b7;
  }

  pre .hljs-variable,
  pre .hljs-attribute,
  pre .hljs-tag,
  pre .hljs-regexp,
  pre .hljs-ruby .constant,
  pre .hljs-xml .tag .title,
  pre .hljs-xml .pi,
  pre .hljs-xml .doctype,
  pre .hljs-html .doctype,
  pre .hljs-css .id,
  pre .hljs-css .class,
  pre .hljs-css .pseudo {
    color: #ff9da4;
  }

  pre .hljs-number,
  pre .hljs-preprocessor,
  pre .hljs-built_in,
  pre .hljs-literal,
  pre .hljs-params,
  pre .hljs-constant {
    color: #ffc58f;
  }

  pre .hljs-class,
  pre .hljs-ruby .class .title,
  pre .hljs-css .rules .attribute {
    color: #ffeead;
  }

  pre .hljs-string,
  pre .hljs-value,
  pre .hljs-inheritance,
  pre .hljs-header,
  pre .hljs-ruby .symbol,
  pre .hljs-xml .cdata {
    color: #d1f1a9;
  }

  pre .hljs-css .hexcolor {
    color: #99ffff;
  }

  pre .hljs-function,
  pre .hljs-python .decorator,
  pre .hljs-python .title,
  pre .hljs-ruby .function .title,
  pre .hljs-ruby .title .keyword,
  pre .hljs-perl .sub,
  pre .hljs-javascript .title,
  pre .hljs-coffeescript .title {
    color: #bbdaff;
  }

  pre .hljs-keyword,
  pre .hljs-javascript .function {
    color: #ebbbff;
  }

  pre code {
    display: block;
    color: white;
    font-family: Menlo, Monaco, Consolas, monospace;
    font-size: 14px;
    line-height: 26px;
    border-radius: 10px;
  }
`;
