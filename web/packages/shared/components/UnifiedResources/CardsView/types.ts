/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import { Ref } from 'react';

// eslint-disable-next-line no-restricted-imports -- TODO(bl-nero)
import { ResourceLabel } from 'teleport/services/agents';

import {
  PinningSupport,
  ResourceLabelConfig,
  UnifiedResourceViewItem,
  VisibleResourceItemFields,
} from '../types';

export type ResourceCardBodyProps = {
  viewItem: UnifiedResourceViewItem;
  visibleInputFields: VisibleResourceItemFields;
  hovered: boolean;
  showResourceSelectedIcon?: boolean | ((labels: ResourceLabel[]) => boolean);
  showAllLabels: boolean;
  labelsInnerContainer: Ref<HTMLDivElement>;
  shouldDisplayStatusWarning: boolean;
  showMoreLabelsButton: boolean;
  onMoreLabelsClick(): void;
  numMoreLabels: number;
  resourceLabelConfig?: ResourceLabelConfig;
  onLabelClick?(label: ResourceLabel): void;
};

export type CardLayoutProps = ResourceCardBodyProps & {
  pinningSupport: PinningSupport;
  pinned: boolean;
  pinResource(): void;
  selectResource(): void;
  selected: boolean;
  onShowStatusInfo(): void;
  showingStatusInfo: boolean;
  setHovered(hovered: boolean): void;
  innerContainer: Ref<HTMLDivElement>;
  onMouseLeave(): void;
};
