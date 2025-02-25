/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import Box from 'design/Box';
import Text, { H2 } from 'design/Text';

/**
 * Renders a header with an optional step counter that appears if there's more
 * than one step. Useful inside a StepSlider, but can be used independently.
 */
export function StepHeader({
  stepIndex,
  flowLength,
  title,
}: {
  stepIndex: number;
  flowLength: number;
  title: string;
}) {
  return (
    <Box>
      {flowLength > 1 && (
        <Text typography="body2" color="text.slightlyMuted">
          Step {stepIndex + 1} of {flowLength}
        </Text>
      )}
      <H2>{title}</H2>
    </Box>
  );
}
