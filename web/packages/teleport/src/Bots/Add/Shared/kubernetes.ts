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

/**
 * `makeKubernetesAccessChecker` creates a checker from the provided labels. Regexes are
 * compiled and reused for subsequent checks.
 *
 * Mimics the backend logic and should be kept in sync;
 *  - lib/services/role.go:1204
 *
 * Label names are always treated as literals and must match fully.
 *
 * Label values can be literals, wildcards or regex;
 * - literals - must match fully (e.g. "dev" matches "dev")
 * - wildcards - can use * to match chunks of zero of more characters (e.g. "us-*-*" matches "us-west-1" and "us-east-2")
 * - regex - regular expression match (e.g. "^(uat|prod)-[0-9]+$")
 *
 * A special combination of *: * (name: value) matches everything even empty target set.
 */
export function makeKubernetesAccessChecker(selected: KubernetesLabel[]) {
  const items = selected.map(s => {
    return {
      name: s.name,
      orMatches: s.values.map(v => {
        let wildcard: RegExp | null = null;
        let regex: RegExp | null = null;
        if (v.includes('*')) {
          try {
            wildcard = new RegExp(`^${v.replaceAll('*', '.*')}$`);
          } catch (e) {
            if (!(e instanceof SyntaxError)) {
              throw e;
            }
            // Ignore regex syntax errors
          }
        } else {
          try {
            regex = new RegExp(v);
          } catch (e) {
            if (!(e instanceof SyntaxError)) {
              throw e;
            }
            // Ignore regex syntax errors
          }
        }
        return {
          literal: v,
          wildcard,
          regex,
        };
      }),
    };
  });
  const hasFullWildcard = selected.some(
    s => s.name === '*' && s.values.some(v => v === '*')
  );
  return {
    check(labels: { name: string; value: string }[]) {
      if (hasFullWildcard) {
        return true;
      }
      return (
        items.length > 0 &&
        items.every(cs => {
          const label = labels.find(l => l.name === cs.name);
          return (
            !!label &&
            cs.orMatches.some(c => {
              const match =
                c.literal === label.value ||
                c.wildcard?.test(label.value) ||
                c.regex?.test(label.value);
              return match;
            })
          );
        })
      );
    },
  };
}

export type KubernetesLabel = { name: string; values: string[] };

export type KubernetesResourceRule = {
  kind: string;
  name: string;
  namespace: string;
  verbs: string[];
};
