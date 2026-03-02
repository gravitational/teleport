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

import {
  getResourcePretitle,
  SelectResourceSpec,
} from 'teleport/Discover/SelectResource/resources';
import { ResourceKind } from 'teleport/Discover/Shared/ResourceKind';

// kindLabelPlural returns the plural form of kindLabel, used as section
// headings in the generated output.
export function kindLabelPlural(kind: ResourceKind): string {
  switch (kind) {
    case ResourceKind.Application:
      return 'Applications';
    case ResourceKind.Database:
      return 'Databases';
    case ResourceKind.Desktop:
      return 'Desktops';
    case ResourceKind.Kubernetes:
      return 'Kubernetes';
    case ResourceKind.Server:
      return 'Servers';
    case ResourceKind.ConnectMyComputer:
      return 'Connect My Computer';
    case ResourceKind.SamlApplication:
      return 'SAML Applications';
    case ResourceKind.MCP:
      return 'MCP Servers';
    default:
      return 'Other';
  }
}

// createGuidedResourceList returns the text of an MDX include file listing
// all resources with guided enrollment flows, grouped by type and sorted by
// type then name. Each type becomes a section heading with its own table.
export function createGuidedResourceList(
  resources: SelectResourceSpec[]
): string {
  // The intention of this list is to include guided enrollment flows for
  // resources in a user's infrastructure so they can set up a Teleport
  // cluster quickly while following the docs.
  // Exclude resources without a Web UI flow. Also exclude Connect My Computer,
  // which is a guided demo flow, not a target infrastructure resource.
  const guided = resources.filter(
    r => r.kind !== ResourceKind.ConnectMyComputer && !r.unguidedLink
  );
  guided.sort((a, b) => {
    const labelA = kindLabelPlural(a.kind);
    const labelB = kindLabelPlural(b.kind);
    if (labelA !== labelB) {
      return labelA.localeCompare(labelB);
    }
    return a.name.localeCompare(b.name);
  });

  const groups = new Map<ResourceKind, SelectResourceSpec[]>();
  for (const r of guided) {
    const existing = groups.get(r.kind);
    if (existing) {
      existing.push(r);
    } else {
      groups.set(r.kind, [r]);
    }
  }

  const sections = Array.from(groups.entries()).map(([kind, members]) => {
    const rows = members
      .map(r => `| ${r.name} | ${getResourcePretitle(r) || 'N/A'} |`)
      .join('\n');
    return `### ${kindLabelPlural(kind)}\n\n| Resource | Deployment Type |\n|----------|----------|\n${rows}`;
  });

  return sections.join('\n\n');
}
