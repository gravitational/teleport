import { screen } from '@testing-library/react';
import type { ComponentProps } from 'react';

import { render as testingRender, userEvent } from 'design/utils/testing';

import {
  MOCK_COMMANDS,
  MOCK_FAILED_COMMAND,
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
    render({ command: MOCK_COMMANDS[1] });

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
    render({ command: MOCK_FAILED_COMMAND });

    expect(screen.getByText('whoami && id')).toBeInTheDocument();
  });

  it('shows error alert in popover for failed command', () => {
    render({ command: MOCK_FAILED_COMMAND, selected: true });

    expect(
      screen.getByText('There was an error analyzing this command:')
    ).toBeInTheDocument();
    expect(
      screen.getByText('temporary fake error for UI testing')
    ).toBeInTheDocument();
  });

  it('hides detailed description when command has errors', () => {
    render({ command: MOCK_FAILED_COMMAND, selected: true });

    expect(
      screen.queryByText(MOCK_FAILED_COMMAND.detailedDescription)
    ).not.toBeInTheDocument();
  });
});

function render(props?: Partial<ComponentProps<typeof TimelineItem>>) {
  return testingRender(
    <TimelineItem
      command={MOCK_COMMANDS[0]}
      selected={false}
      onOpenChange={() => {}}
      onPlay={() => {}}
      {...props}
    />
  );
}
