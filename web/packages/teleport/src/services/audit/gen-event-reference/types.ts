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

/*
 * These are more generalized version of types from services/audit/types.ts.
 * gen-event-reference doesn't care about exact codes. We just need to make sure that the object
 * shapes stay in sync between audit and gen-event-reference.
 */

export type Event = {
  id: string;
  time: Date;
  user: string;
  message: string;
  code: string;
  codeDesc: string;
  raw: RawEvent;
};

export type Formatters = {
  [key in string]: {
    type: string;
    desc: string | ((json: RawEvent) => string);
    format: (json: RawEvent) => string;
  };
};

type RawEvent = {
  code: string;
  user?: string;
  time: string;
  uid: string;
  event: string;
};
