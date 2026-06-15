import { screen } from '@testing-library/react';
import type { ComponentProps } from 'react';

import { render as testingRender, userEvent } from 'design/utils/testing';

import {
  MOCK_DESKTOP_EVENTS,
  MOCK_EVENTS,
  MOCK_FAILED_DESKTOP_EVENT,
  MOCK_FAILED_EVENT,
} from 'e-teleport/SessionRecordings/summary/mock';
import { TimelineItem } from 'e-teleport/SessionRecordings/summary/TimelineItem';

describe('TimelineItem', () => {
  it('renders the timeline title', () => {
    render();

    expect(screen.getByText('Remote Script Execution')).toBeInTheDocument();
  });

  it('renders the formatted time offset', () => {
    render();

    expect(screen.getByText('0:01')).toBeInTheDocument();
  });

  it('formats offset with hours when applicable', () => {
    render({ event: MOCK_EVENTS[1] });

    expect(screen.getByText('0:02')).toBeInTheDocument();
  });

  it('calls onOpenChange when clicked', async () => {
    const onOpenChange = jest.fn();

    render({ onOpenChange });

    await userEvent.click(screen.getByText('Remote Script Execution'));

    expect(onOpenChange).toHaveBeenCalledWith(true);
  });

  it('toggles selection state on click', async () => {
    const onOpenChange = jest.fn();

    render({ selected: true, onOpenChange });

    await userEvent.click(screen.getByText('Remote Script Execution'));

    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it('renders raw command with warning icon when analysis failed', () => {
    render({ event: MOCK_FAILED_EVENT });

    expect(screen.getByText('whoami && id')).toBeInTheDocument();
  });

  it('shows error alert in popover for failed command', () => {
    render({ event: MOCK_FAILED_EVENT, selected: true });

    expect(
      screen.getByText('There was an error analyzing this command:')
    ).toBeInTheDocument();
    expect(
      screen.getByText('temporary fake error for UI testing')
    ).toBeInTheDocument();
  });

  it('hides detailed description when command has errors', () => {
    render({ event: MOCK_FAILED_EVENT, selected: true });

    expect(
      screen.queryByText(MOCK_FAILED_EVENT.detailedDescription)
    ).not.toBeInTheDocument();
  });

  it('shows the command text inside the popover', () => {
    render({ selected: true });

    expect(
      screen.getByText(MOCK_EVENTS[0].commandEventDetails!.command)
    ).toBeInTheDocument();
  });

  it('renders the timeline title of a desktop event', () => {
    render({ event: MOCK_DESKTOP_EVENTS[0] });

    expect(
      screen.getByText('Uploaded Payroll Data to Personal Cloud Storage')
    ).toBeInTheDocument();
  });

  it('shows desktop event details in popover', () => {
    render({ event: MOCK_DESKTOP_EVENTS[0], selected: true });

    expect(screen.getByText('Active window')).toBeInTheDocument();
    expect(screen.getByText('My Drive - Google Chrome')).toBeInTheDocument();

    expect(screen.getByText('Applications')).toBeInTheDocument();
    expect(
      screen.getByText('Google Chrome, File Explorer')
    ).toBeInTheDocument();

    expect(screen.getByText('Visible URLs')).toBeInTheDocument();
    expect(
      screen.getByText('https://drive.google.com/drive/my-drive')
    ).toBeInTheDocument();

    expect(screen.getByText('Visible file paths')).toBeInTheDocument();
    expect(
      screen.getByText('C:\\Users\\Administrator\\Documents\\payroll.xlsx')
    ).toBeInTheDocument();
  });

  it('omits desktop detail sections without values', () => {
    render({ event: MOCK_DESKTOP_EVENTS[1], selected: true });

    expect(screen.getByText('Active window')).toBeInTheDocument();
    expect(screen.queryByText('Visible URLs')).not.toBeInTheDocument();
    expect(screen.queryByText('Visible file paths')).not.toBeInTheDocument();
  });

  it('does not render visible URLs as links', () => {
    render({ event: MOCK_DESKTOP_EVENTS[0], selected: true });

    expect(
      screen.getByText('https://drive.google.com/drive/my-drive')
    ).not.toHaveAttribute('href');
    expect(
      screen.queryByRole('link', {
        name: 'https://drive.google.com/drive/my-drive',
      })
    ).not.toBeInTheDocument();
  });

  it('falls back to the active window title when a desktop event has no timeline title', () => {
    render({ event: MOCK_FAILED_DESKTOP_EVENT });

    expect(screen.getByText('Remote Desktop Connection')).toBeInTheDocument();
  });

  it('shows error alert in popover for failed desktop event', () => {
    render({ event: MOCK_FAILED_DESKTOP_EVENT, selected: true });

    expect(
      screen.getByText('There was an error analyzing this event:')
    ).toBeInTheDocument();
    expect(
      screen.getByText('temporary fake error for UI testing')
    ).toBeInTheDocument();
  });
});

function render(props?: Partial<ComponentProps<typeof TimelineItem>>) {
  return testingRender(
    <TimelineItem
      event={MOCK_EVENTS[0]}
      selected={false}
      onOpenChange={() => {}}
      onPlay={() => {}}
      {...props}
    />
  );
}
