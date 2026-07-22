/**
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

import { Card, CardSuccess, H1 } from 'design';
import { CircleStop } from 'design/Icon';

export function CardDenied({ title, children }) {
  return (
    <Card width="540px" p={7} my={4} mx="auto" textAlign="center">
      <CircleStop mb={3} size={64} color="red" />
      {title && <H1 mb="4">{title}</H1>}
      {children}
    </Card>
  );
}

export function CardAccept({ title, children }) {
  return <CardSuccess title={title}>{children}</CardSuccess>;
}
