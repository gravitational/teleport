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

jest.mock('./TextEditor', () => {
  return {
    __esModule: true,
    default: MockTextEditor,
  };
});

/**
 * How to use this?
 *
 * Import "shared/components/TextEditor/TextEditor.mock" in your test file and
 * the mock will be setup for you. It can be used to test the content only, no
 * other features are available in the mock.
 */

function MockTextEditor(props: { data?: [{ content: string }] }) {
  return (
    <div data-testid="mock-text-editor">
      {props.data?.map(d => (
        <div key={d.content}>{d.content}</div>
      ))}
    </div>
  );
}
