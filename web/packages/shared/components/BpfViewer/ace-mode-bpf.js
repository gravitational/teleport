/*
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

import ace from 'ace-builds/src-min-noconflict/ace';

// Defines custom ace mode module
ace.define(
  'ace/mode/grv_bpf',
  [
    'require',
    'exports',
    'ace/lib/oop',
    'ace/mode/text',
    'ace/mode/matching_brace_outdent',
    'ace/mode/grv_bpf_highlight_rules',
  ],
  (acequire, exports) => {
    const oop = acequire('../lib/oop');
    const TextMode = acequire('./text').Mode;
    const FoldMode = acequire('./folding/coffee').FoldMode;
    const MatchingBraceOutdent = acequire(
      './matching_brace_outdent'
    ).MatchingBraceOutdent;

    const GravitationalHighlightRules = acequire(
      'ace/mode/grv_bpf_highlight_rules'
    ).GravitationalHighlightRules;

    const Mode = function () {
      this.$outdent = new MatchingBraceOutdent();
      this.foldingRules = new FoldMode();
      this.HighlightRules = GravitationalHighlightRules;
    };

    oop.inherits(Mode, TextMode);
    exports.Mode = Mode;
  }
);

// This is where we really create the highlighting rules
ace.define(
  'ace/mode/grv_bpf_highlight_rules',
  ['require', 'exports', 'ace/lib/oop', 'ace/mode/text_highlight_rules'],
  (acequire, exports) => {
    const oop = acequire('ace/lib/oop');
    const TextHighlightRules = acequire(
      'ace/mode/text_highlight_rules'
    ).TextHighlightRules;

    function GravitationalHighlightRules() {
      this.$rules = new TextHighlightRules().getRules(); // Use Text's rules as a base
      this.$rules.start = [
        {
          token: ['grv_file.variable', 'grv_space.text', 'grv_path.string'],
          regex: /(file)(\t+)(.*)/,
        },
        {
          token: [
            'grv_network.variable',
            'grv_space.text',
            'grv_addr.constant.numeric',
            'grv_space.text',
            'grv_addr.constant.numeric',
            'grv_space.text',
            'grv_addr.constant.numeric',
          ],
          regex: /(network)(\t+)([^\t]+)?(\t+)(->)(\t+)([^\t]+)?/,
        },
        {
          token: createTokens,
          regex: /(?!\t)([^\t]+)?(\t+)([^\t]+)?(\t+)([^\t]+)?(\t+)(.*)/,
        },
        {
          token: createTokens,
          regex: /(?!\t)([^\t]+)?(\t+)([^\t]+)?(\t+)([^\t]+)/,
        },
        {
          token: 'string', // multi line string start
          regex: /[|>][-+\d]*(?:$|\s+(?:$|#))/,
          onMatch: function (val, state, stack, line) {
            line = line.replace(/ #.*/, '');
            var indent = /^ *((:\s*)?-(\s*[^|>])?)?/
              .exec(line)[0]
              .replace(/\S\s*$/, '').length;
            var indentationIndicator = parseInt(/\d+[\s+-]*$/.exec(line));

            if (indentationIndicator) {
              indent += indentationIndicator - 1;
              this.next = 'mlString';
            } else {
              this.next = 'mlStringPre';
            }
            if (!stack.length) {
              stack.push(this.next);
              stack.push(indent);
            } else {
              stack[0] = this.next;
              stack[1] = indent;
            }
            return this.token;
          },
          next: 'mlString',
        },
      ];

      this.$rules.mlStringPre = [
        {
          token: 'indent',
          regex: /^ *$/,
        },
        {
          token: 'indent',
          regex: /^ */,
          onMatch: function (val, state, stack) {
            var curIndent = stack[1];

            if (curIndent >= val.length) {
              this.next = 'start';
              stack.shift();
              stack.shift();
            } else {
              stack[1] = val.length - 1;
              this.next = stack[0] = 'mlString';
            }
            return this.token;
          },
          next: 'mlString',
        },
        {
          defaultToken: 'string',
        },
      ];

      this.$rules.mlString = [
        {
          token: 'indent',
          regex: /^ *$/,
        },
        {
          token: 'indent',
          regex: /^ */,
          onMatch: function (val, state, stack) {
            var curIndent = stack[1];

            if (curIndent >= val.length) {
              this.next = 'start';
              stack.splice(0);
            } else {
              this.next = 'mlString';
            }
            return this.token;
          },
          next: 'mlString',
        },
        {
          token: 'string',
          regex: '.+',
        },
      ];

      this.normalizeRules();
    }

    function createTokens(...args) {
      if (args.length === 3) {
        return ['grv_date.comment', 'grv_space.text', 'grv_path.string'];
      }

      // file
      if (args.length === 4) {
        return [
          'grv_date.comment',
          'grv_space.text',
          'grv_path.string',
          'grv_space.text',
        ];
      }

      // network
      if (args.length === 5) {
        return [
          'grv_date.comment',
          'grv_space.text',
          'grv_path.string',
          'grv_space.text',
          'grv_cmd.keyword',
        ];
      }

      // command
      return [
        'grv_date.comment',
        'grv_space.text',
        'grv_path.string',
        'grv_space.text',
        'grv_cmd.keyword',
        'grv_space.text',
        'grv_space.text',
      ];
    }

    oop.inherits(GravitationalHighlightRules, TextHighlightRules);

    exports.GravitationalHighlightRules = GravitationalHighlightRules;
  }
);
