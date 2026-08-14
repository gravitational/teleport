import { render, screen, userEvent } from 'design/utils/testing';

import { RecoveryCodesProps } from 'teleport/components/RecoveryCodes';

import { RecoveryCodes } from './RecoveryCodes';

function TestDialog(props: Partial<RecoveryCodesProps>) {
  return (
    <RecoveryCodes
      recoveryCodes={{
        codes: ['code1', 'code2', 'code3'],
        createdDate: new Date('2019-08-30T11:00:00.00Z'),
      }}
      isNewCodes={true}
      continueText="Continue"
      onContinue={() => {}}
      {...props}
    />
  );
}

test('renders basic information', () => {
  render(<TestDialog isNewCodes={false} />);
  expect(screen.getByText('code1 code2 code3')).toBeVisible();
  expect(screen.getByText('1. Why do I need these codes?')).toBeVisible();
  expect(screen.getByText('2. How long do the codes last for?')).toBeVisible();
  expect(
    screen.queryByText('3. What about my old codes?')
  ).not.toBeInTheDocument();
});

test('renders additional FAQ after regenerating codes', () => {
  render(<TestDialog isNewCodes={true} />);
  expect(screen.getByText('3. What about my old codes?')).toBeVisible();
});

test('confirming that recovery codes were saved is required to continue', async () => {
  const user = userEvent.setup();
  const onContinue = jest.fn();
  render(<TestDialog onContinue={onContinue} />);

  await user.click(screen.getByRole('button', { name: 'Continue' }));
  expect(onContinue).not.toHaveBeenCalled();

  await user.click(
    screen.getByRole('checkbox', { name: 'I have saved my new recovery codes' })
  );
  await user.click(screen.getByRole('button', { name: 'Continue' }));
  expect(onContinue).toHaveBeenCalled();
});
