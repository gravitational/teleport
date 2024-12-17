import { render } from 'design/utils/testing';

import { FromInvite, FromReset } from './RecoveryCodes.story';

// TODO(bl-nero): Snapshot tests removed. Tests to be replaced by Storybook
// snapshot tests. See https://github.com/gravitational/teleport/issues/19185.

test('render correct dialog after creating a new account', () => {
  render(<FromInvite />);
});

test('render correct dialog after resetting account', () => {
  render(<FromReset />);
});
