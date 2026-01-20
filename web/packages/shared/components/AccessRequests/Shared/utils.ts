/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import { formatDuration, intervalToDuration } from 'date-fns';

import {
  ResourceConstraints,
  ResourceConstraintsVariant,
  ResourceIDString,
} from 'shared/services/accessRequests';

import { ResourceMap } from '../NewRequest';

export function getFormattedDurationTxt({
  start,
  end,
}: {
  start: Date;
  end: Date;
}) {
  return formatDuration(intervalToDuration({ start, end }), {
    format: ['weeks', 'days', 'hours', 'minutes'],
  });
}

export function getNumAddedResources(addedResources: ResourceMap) {
  return (
    Object.keys(addedResources.node).length +
    Object.keys(addedResources.db).length +
    Object.keys(addedResources.app).length +
    Object.keys(addedResources.kube_cluster).length +
    Object.keys(addedResources.user_group).length +
    Object.keys(addedResources.windows_desktop).length +
    Object.keys(addedResources.saml_idp_service_provider).length +
    Object.keys(addedResources.namespace).length +
    Object.keys(addedResources.aws_ic_account_assignment).length
  );
}

const AWS_IAM_ROLE_ARN_REGEX = /^arn:aws[a-z0-9-]*:iam::(\d{12}):role\/(.+)$/;

/**
 * Formats an AWS Role ARN for pretty display, in the format "accountId: rolePathAndName".
 */
export const formatAWSRoleARNForDisplay = (arn: string) => {
  const match = arn.match(AWS_IAM_ROLE_ARN_REGEX);

  if (!match || match.length < 3) {
    return arn;
  }

  const [, accountId, rolePathAndName] = match;

  return `${accountId}: ${rolePathAndName}`;
};

/**
 * Toggles an AWS Console constraint by removing the specified ARN from the current constraints.
 * If no RoleARNs remain after removal, it clears the constraint.
 */
export const toggleAWSConsoleConstraint =
  (
    key: ResourceIDString,
    curr: ResourceConstraintsVariant<'aws_console'>,
    set?: (
      key: ResourceIDString,
      constraints: ResourceConstraints | undefined
    ) => void
  ) =>
  (arn: string) => {
    const newRc = {
      aws_console: {
        role_arns: curr.aws_console.role_arns.filter(a => a !== arn),
      },
    };
    set?.(key, newRc.aws_console.role_arns.length ? newRc : undefined);
  };
