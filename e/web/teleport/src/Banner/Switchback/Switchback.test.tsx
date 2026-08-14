import { render, screen, userEvent } from 'design/utils/testing';

import { Switchback } from './Switchback';

const TestSwitchback = ({ attempt, onSwitchBack, onErrorConfirm }) => (
  <Switchback
    assumedRoles={['auditor', 'approver']}
    time={{
      hours: 1,
      minutes: 2,
      seconds: 3,
    }}
    btnSetting={{
      text: 'Switch Back',
      func: onSwitchBack,
    }}
    attempt={attempt}
    onErrorConfirm={onErrorConfirm}
  />
);

test('renders and dismisses the banner', async () => {
  const user = userEvent.setup();
  const onSwitchBack = jest.fn();
  render(
    <TestSwitchback
      attempt={{ status: '', statusText: '' }}
      onSwitchBack={onSwitchBack}
      onErrorConfirm={() => {}}
    />
  );

  expect(screen.getByText(/auditor, approver/)).toBeVisible();
  expect(screen.getByText(/1 hr and 2 mins/)).toBeVisible();

  await user.click(screen.getByRole('button', { name: 'Switch Back' }));
  expect(onSwitchBack).toHaveBeenCalled();
});

test('renders and confirms the error dialog', async () => {
  const user = userEvent.setup();
  const onErrorConfirm = jest.fn();
  render(
    <TestSwitchback
      attempt={{ status: 'failed', statusText: 'Oh noes' }}
      onSwitchBack={() => {}}
      onErrorConfirm={onErrorConfirm}
    />
  );

  expect(screen.getByText('Oh noes')).toBeVisible();

  await user.click(screen.getByRole('button', { name: 'OK' }));
  expect(onErrorConfirm).toHaveBeenCalled();
});
