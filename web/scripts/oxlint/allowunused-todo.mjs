/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

// This plugin ensures that @allowunused tags are JSDoc comments with a TODO(owner): reason format,
// so knip can read them and they don't accumulate without an owner to remove them.

const GITHUB_USERNAME = '[A-Za-z\\d](?:[A-Za-z\\d]|-(?=[A-Za-z\\d])){0,38}';
const VALID_ALLOWUNUSED = new RegExp(
  `^@allowunused[^\\S\\r\\n]+TODO\\(${GITHUB_USERNAME}\\):[^\\S\\r\\n]*\\S`
);

const allowunusedTodo = {
  meta: {
    docs: {
      description:
        'Require @allowunused knip suppressions to be JSDoc comments with an owner: /** @allowunused TODO(github-username): reason */',
    },
    messages: {
      notJsdoc:
        'knip only reads @allowunused from a /** JSDoc */ comment attached to the export, it has no effect here.',
      badFormat:
        '@allowunused must name an owner responsible for removing it: /** @allowunused TODO(github-username): reason */',
    },
  },
  create(context) {
    return {
      Program() {
        for (const comment of context.sourceCode.getAllComments()) {
          const isJsdoc =
            comment.type === 'Block' && comment.value.startsWith('*');

          if (!isJsdoc) {
            if (/^\s*@allowunused/i.test(comment.value)) {
              context.report({ loc: comment.loc, messageId: 'notJsdoc' });
            }
            continue;
          }
          if (
            comment.value
              .matchAll(/@allowunused/gi)
              .some(
                match =>
                  !VALID_ALLOWUNUSED.test(comment.value.slice(match.index))
              )
          ) {
            context.report({ loc: comment.loc, messageId: 'badFormat' });
          }
        }
      },
    };
  },
};

export default {
  meta: { name: 'teleport' },
  rules: { 'allowunused-todo': allowunusedTodo },
};
