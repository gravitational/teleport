/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

/**
 * openNavSection opens a sidenav section and returns its locator
 */
export async function openNavSection(
  page: import('@playwright/test').Page,
  sectionName: string
) {
  const button = page.getByRole('button', { name: sectionName });
  await button.hover();
  await button.click();
  const panel = page.locator(`[id="panel-${sectionName}"]`);
  await panel.waitFor({ state: 'visible', timeout: 5_000 });
  return panel;
}