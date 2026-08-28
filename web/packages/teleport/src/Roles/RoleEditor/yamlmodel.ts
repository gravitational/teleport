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

export type YamlEditorModel = {
  content: string;
  /**
   * will be true if initial yaml
   * content were modified
   */
  isDirty: boolean;
  /**
   * Indicates if the user interacted with the YAML editor. It's different from
   * {@link YamlEditorModel.isDirty} by not taking into consideration if the
   * content is different from the original.
   */
  isTouched: boolean;
};
