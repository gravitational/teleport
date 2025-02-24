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

import ButtonSso from './ButtonSso';

export default {
  title: 'Shared/ButtonSso',
};

export const Button = () => (
  <div style={{ width: '300px' }}>
    <ButtonSso mt={3} title="Valdez" ssoType="microsoft" />
    <ButtonSso mt={3} title="Norman" ssoType="github" />
    <ButtonSso mt={3} title="Barnes" ssoType="google" />
    <ButtonSso mt={3} title="Norton" ssoType="bitbucket" />
    <ButtonSso mt={3} title="Russell" ssoType="unknown" />
  </div>
);

Button.storyName = 'ButtonSso';
