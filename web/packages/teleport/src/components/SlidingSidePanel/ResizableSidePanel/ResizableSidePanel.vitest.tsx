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

import { expect, test } from 'vitest';

import { fireEvent, render, screen, userEvent } from 'design/utils/testing';

import { getPanelTitle, ResizablePanelLayout, ResizableSidePanel } from '.';
import { SidePanelTab, SidePanelTabs } from './SidePanelTab';
import { useResizableSidePanel } from './useResizableSidePanel';

test('closes and reopens at the last set width', async () => {
  render(<TestPanel />);

  const panel = screen.getByTestId('resizable-side-panel');
  const resizeHandle = panel.firstElementChild;

  // Resize panel - the panel is "right" aligned, so we have
  // to drag towards the left side to make it "wider"
  fireEvent.mouseDown(resizeHandle, { clientX: 500 });
  fireEvent.mouseMove(window, { clientX: 400 });
  fireEvent.mouseUp(window);

  expect(panel).toHaveStyle({ width: '600px' });

  await userEvent.click(
    screen.getByRole('button', { name: 'Close side panel' })
  );
  expect(screen.queryByTestId('resizable-side-panel')).not.toBeInTheDocument();

  await userEvent.click(screen.getByRole('button', { name: 'Info Guide' }));
  expect(screen.getByTestId('resizable-side-panel')).toHaveStyle({
    width: '600px',
  });
});

test('switching tabs and closing the active tab when toggled', async () => {
  render(<TestPanel />);

  // Defaults to info tab and panel.
  const infoButton = screen.getByRole('button', { name: 'Info Guide' });
  const terraformButton = screen.getByRole('button', {
    name: 'Terraform',
  });

  expect(infoButton).toHaveAttribute('aria-pressed', 'true');
  expect(screen.getByText('Info panel content')).toBeInTheDocument();

  // Click on terraform tab.
  await userEvent.click(terraformButton);

  expect(infoButton).toHaveAttribute('aria-pressed', 'false');
  expect(terraformButton).toHaveAttribute('aria-pressed', 'true');
  expect(screen.getByText('Terraform panel content')).toBeInTheDocument();

  // Clicking on the same tab closes the panel.
  await userEvent.click(terraformButton);

  expect(screen.queryByTestId('resizable-side-panel')).not.toBeInTheDocument();
  expect(terraformButton).toHaveAttribute('aria-pressed', 'false');
});

test('clamping of the panel width when resizing', () => {
  render(<TestPanel />);

  let panel = screen.getByTestId('resizable-side-panel');
  let resizeHandle = panel.firstElementChild;

  // Resizing it out of min width range, defaults to set minWidth
  fireEvent.mouseDown(resizeHandle, { clientX: 500 });
  fireEvent.mouseMove(window, { clientX: 1000 });
  fireEvent.mouseUp(window);

  expect(panel).toHaveStyle({ width: '365px' });

  panel = screen.getByTestId('resizable-side-panel');
  resizeHandle = panel.firstElementChild;

  // Resizing it out of max width range, defaults to set maxWidth
  fireEvent.mouseDown(resizeHandle, { clientX: 500 });
  fireEvent.mouseMove(window, { clientX: -500 });
  fireEvent.mouseUp(window);

  expect(panel).toHaveStyle({ width: '900px' });
});

function TestPanel() {
  const panel = useResizableSidePanel({
    defaultWidth: 500,
    minWidth: 365,
    maxWidth: 900,
  });

  return (
    <ResizablePanelLayout>
      <div>some main content</div>
      <SidePanelTabs>
        <SidePanelTab
          ariaLabel={getPanelTitle('info')}
          isActive={panel.isOpen && panel.activeTab === 'info'}
          onClick={() => panel.toggleTab('info')}
        >
          Fake info icon
        </SidePanelTab>
        <SidePanelTab
          ariaLabel={getPanelTitle('terraform')}
          isActive={panel.isOpen && panel.activeTab === 'terraform'}
          onClick={() => panel.toggleTab('terraform')}
        >
          Fake terraform icon
        </SidePanelTab>
      </SidePanelTabs>
      {panel.isOpen && (
        <ResizableSidePanel
          onClose={panel.close}
          onWidthChange={panel.updateWidth}
          title={getPanelTitle(panel.activeTab)}
          width={panel.width}
        >
          {panel.activeTab === 'info'
            ? 'Info panel content'
            : 'Terraform panel content'}
        </ResizableSidePanel>
      )}
    </ResizablePanelLayout>
  );
}
