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

import { ResourceMap } from 'shared/components/AccessRequests/NewRequest';
import {
  getResourceIDString,
  hasResourceConstraints,
  ResourceConstraints,
  ResourceConstraintsKind,
  ResourceIDString,
  WithResourceConstraints,
} from 'shared/services/accessRequests';

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

type ResourceIdentifiable = { id: string; kind: string; clusterName: string };

type ToggleConstraintFn = (
  key: ResourceIDString,
  rc?: ResourceConstraints
) => void;

/** A read-only display section for a constraint dimension. */
export type ConstraintSection = {
  title: string;
  values: string[];
  formatLabel?: (v: string) => string;
};

/** A display section that also supports interactive removal. */
export type EditableConstraintSection = ConstraintSection & {
  onRemove: (value: string) => void;
};

const getKeyFromItem = (item: ResourceIdentifiable): ResourceIDString =>
  getResourceIDString({
    name: item.id,
    kind: item.kind,
    cluster: item.clusterName,
  });

/** Removes an ARN from an AWS Console constraint, clearing it if empty. */
export const toggleAWSConsoleConstraint = <T extends ResourceIdentifiable>(
  item: WithResourceConstraints<'aws_console', T>,
  arn: string,
  set: ToggleConstraintFn
) => {
  const newArns = item.constraints.aws_console.role_arns.filter(a => a !== arn);
  set(
    getKeyFromItem(item),
    newArns.length ? { aws_console: { role_arns: newArns } } : undefined
  );
};

/** Removes a login from an SSH constraint, clearing it if empty. */
export const toggleSSHConstraint = <T extends ResourceIdentifiable>(
  item: WithResourceConstraints<'ssh', T>,
  login: string,
  set: ToggleConstraintFn
) => {
  const newLogins = item.constraints.ssh.logins.filter(l => l !== login);
  set(
    getKeyFromItem(item),
    newLogins.length ? { ssh: { logins: newLogins } } : undefined
  );
};

/**
 * Maps each Resource Constraint variant to a function that extracts
 * sections for display.
 */
const constraintSectionExtractors: {
  [K in ResourceConstraintsKind]: (
    item: WithResourceConstraints<K, ResourceIdentifiable>,
    toggleFn?: ToggleConstraintFn
  ) => (ConstraintSection | EditableConstraintSection)[];
} = {
  aws_console: (item, toggleFn) => [
    {
      title: 'Role ARNs',
      values: item.constraints.aws_console.role_arns,
      formatLabel: formatAWSRoleARNForDisplay,
      ...(toggleFn && {
        onRemove: (v: string) => toggleAWSConsoleConstraint(item, v, toggleFn),
      }),
    },
  ],
  ssh: (item, toggleFn) => [
    {
      title: 'SSH Logins',
      values: item.constraints.ssh.logins,
      ...(toggleFn && {
        onRemove: (v: string) => toggleSSHConstraint(item, v, toggleFn),
      }),
    },
  ],
};

const constraintKinds = Object.keys(
  constraintSectionExtractors
) as ResourceConstraintsKind[];

/** Extracts display sections for rendering a resource's constraints. */
export function getResourceConstraintSections(item: {
  constraints?: ResourceConstraints;
}): ConstraintSection[];
export function getResourceConstraintSections(
  item: ResourceIdentifiable & { constraints?: ResourceConstraints },
  toggleFn: ToggleConstraintFn
): EditableConstraintSection[];
export function getResourceConstraintSections(
  item: ResourceIdentifiable & { constraints?: ResourceConstraints },
  toggleFn?: ToggleConstraintFn
): (ConstraintSection | EditableConstraintSection)[] {
  const sections: (ConstraintSection | EditableConstraintSection)[] = [];
  for (const kind of constraintKinds) {
    if (hasResourceConstraints(item, kind)) {
      sections.push(...constraintSectionExtractors[kind](item, toggleFn));
    }
  }
  return sections;
}
