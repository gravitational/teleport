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

import CardIcon from 'design/CardIcon';
import { CircleCheck } from 'design/Icon';

export default function CardSuccess({ title, children }) {
  return (
    <CardIcon
      title={title}
      icon={<CircleCheck mb={3} size={64} color="success.main" />}
    >
      {children}
    </CardIcon>
  );
}

export function CardSuccessLogin() {
  return (
    <CardSuccess title="Login Successful">
      You have successfully signed into your account. <br /> You can close this
      window and continue using the product.
    </CardSuccess>
  );
}
