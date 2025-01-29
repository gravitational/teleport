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

import { Box, H2 } from 'design';

export default function CardEmpty(props) {
  const { title, children, ...styles } = props;
  return (
    <Box
      bg="levels.surface"
      p="4"
      width="100%"
      m="0 auto"
      maxWidth="600px"
      textAlign="center"
      color="text.main"
      style={{ borderRadius: '6px' }}
      {...styles}
    >
      <H2>{title}</H2>
      {children}
    </Box>
  );
}
