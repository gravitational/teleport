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

import cfg from 'teleport/config';
import api from 'teleport/services/api';

import {
  YamlParseRequest,
  YamlStringifyRequest,
  YamlSupportedResourceKind,
} from './types';

export const yamlService = {
  parse<T>(kind: YamlSupportedResourceKind, req: YamlParseRequest): Promise<T> {
    return api.post(cfg.getYamlParseUrl(kind), req).then(resp => resp.resource);
  },

  stringify<T>(
    kind: YamlSupportedResourceKind,
    req: YamlStringifyRequest<T>
  ): Promise<string> {
    return api.post(cfg.getYamlStringifyUrl(kind), req).then(resp => resp.yaml);
  },
};
