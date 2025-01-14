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

import styled from 'styled-components';

import Link from 'design/Link';
import { MarkInverse } from 'design/Mark';
import { Position } from 'design/Popover/Popover';
import { IconTooltip } from 'design/Tooltip';

/**
 * Returns a IconTooltip component with its tip contents
 * set to the requested resource kind.
 *
 * @param resourceKind - the tip contents differ slightly depending
 * on the resource kind
 * @param toolTipPosition (opt) - the position which the tooltip should
 * render (see type Position)
 * @returns JSX Element
 */
export function ResourceLabelTooltip({
  resourceKind,
  toolTipPosition,
}: {
  resourceKind: 'server' | 'eks' | 'rds' | 'kube' | 'db' | 'app';
  toolTipPosition?: Position;
}) {
  let tip;

  switch (resourceKind) {
    case 'app': {
      tip = (
        <>
          Labels allow you to do the following:
          <Ul>
            <li>
              Filter applications by labels when using tsh, tctl, or the web UI.
            </li>
            <li>
              Restrict access to this application with{' '}
              <Link
                target="_blank"
                href="https://goteleport.com/docs/enroll-resources/application-access/controls/"
              >
                Teleport RBAC
              </Link>
              . Only roles with <MarkInverse>app_labels</MarkInverse> that match
              these labels will be allowed to access this application.
            </li>
          </Ul>
        </>
      );
      break;
    }
    case 'server': {
      tip = (
        <>
          Labels allow you to do the following:
          <Ul>
            <li>
              Filter servers by labels when using tsh, tctl, or the web UI.
            </li>
            <li>
              Restrict access to this server with{' '}
              <Link
                target="_blank"
                href="https://goteleport.com/docs/enroll-resources/server-access/rbac/"
              >
                Teleport RBAC
              </Link>
              . Only roles with <MarkInverse>node_labels</MarkInverse> that
              match these labels will be allowed to access this server.
            </li>
          </Ul>
        </>
      );
      break;
    }
    case 'kube':
    case 'eks': {
      tip = (
        <>
          Labels allow you to do the following:
          <Ul>
            <li>
              Filter Kubernetes clusters by labels when using tsh, tctl, or the
              web UI.
            </li>
            <li>
              Restrict access to this Kubernetes cluster with{' '}
              <Link
                target="_blank"
                href="https://goteleport.com/docs/enroll-resources/kubernetes-access/controls/"
              >
                Teleport RBAC
              </Link>
              . Only roles with <MarkInverse>kubernetes_labels</MarkInverse>{' '}
              that match these labels will be allowed to access this Kubernetes
              cluster.
            </li>
            {resourceKind === 'eks' && (
              <li>
                All the AWS tags from the selected EKS will be included upon
                enrollment.
              </li>
            )}
          </Ul>
        </>
      );
      break;
    }
    case 'rds':
    case 'db': {
      tip = (
        <>
          Labels allow you to do the following:
          <Ul>
            <li>
              Filter databases by labels when using tsh, tctl, or the web UI.
            </li>
            <li>
              Restrict access to this database with{' '}
              <Link
                target="_blank"
                href="https://goteleport.com/docs/enroll-resources/database-access/rbac/"
              >
                Teleport RBAC
              </Link>
              . Only roles with <MarkInverse>db_labels</MarkInverse> that match
              these labels will be allowed to access this database.
            </li>
            {resourceKind === 'rds' && (
              <li>
                All the AWS tags from the selected RDS will be included upon
                enrollment.
              </li>
            )}
          </Ul>
        </>
      );
      break;
    }
    default:
      resourceKind satisfies never;
  }

  return (
    <IconTooltip sticky={true} position={toolTipPosition}>
      {tip}
    </IconTooltip>
  );
}

const Ul = styled.ul`
  margin: 0;
  padding-left: ${p => p.theme.space[4]}px;
`;
