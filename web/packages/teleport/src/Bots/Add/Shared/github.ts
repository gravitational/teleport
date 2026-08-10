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

import { Option } from 'shared/components/Select';
import { Rule } from 'shared/components/Validation/rules';

import { RefType } from 'teleport/services/bot/types';

export const GITHUB_HOST = 'github.com';

export type RefTypeOption = Option<RefType | ''>;

export const refTypeOptions: RefTypeOption[] = [
  {
    label: 'any',
    value: '',
  },
  {
    label: 'Branch',
    value: 'branch',
  },
  {
    label: 'Tag',
    value: 'tag',
  },
];

/**
 * Parses the GitHub repository URL and returns the repository name and
 * its owner's name. Throws errors if parsing the URL fails or
 * the URL doesn't contains the expected format.
 * @param repoAddr repository address (with or without protocl)
 * @returns owner and repository name
 */
export function parseRepoAddress(repoAddr: string): {
  host: string;
  owner: string;
  repository: string;
} {
  // add protocol if it is missing
  if (!repoAddr.startsWith('http://') && !repoAddr.startsWith('https://')) {
    repoAddr = `https://${repoAddr}`;
  }

  let url: URL;
  try {
    url = new URL(repoAddr);
  } catch (error) {
    throw new Error('Must be a valid URL', { cause: error });
  }

  const paths = url.pathname.split('/');
  // expected length is 3, since pathname starts with a /, so paths[0] should be empty
  if (paths.length < 3) {
    throw new Error(
      'URL expected to be in the format https://<host>/<owner>/<repository>'
    );
  }

  const owner = paths[1];
  const repository = paths[2];
  if (owner.trim() === '' || repository.trim() == '') {
    throw new Error(
      'URL expected to be in the format https://<host>/<owner>/<repository>'
    );
  }

  return {
    host: url.host,
    owner,
    repository,
  };
}

export const requireValidRepository: Rule = value => () => {
  if (!value) {
    return { valid: false, message: 'Repository is required' };
  }
  let repoAddr = value.trim();
  if (!repoAddr) {
    return { valid: false, message: 'Repository is required' };
  }

  // add protocol if user omited it
  if (!repoAddr.startsWith('http://') && !repoAddr.startsWith('https://')) {
    repoAddr = `https://${repoAddr}`;
  }

  try {
    const { owner, repository } = parseRepoAddress(repoAddr);
    if (owner.trim() === '' || repository.trim() == '') {
      return {
        valid: false,
        message:
          'URL expected to be in the format https://<host>/<owner>/<repository>',
      };
    }

    return { valid: true };
  } catch (e) {
    return { valid: false, message: e?.message };
  }
};
