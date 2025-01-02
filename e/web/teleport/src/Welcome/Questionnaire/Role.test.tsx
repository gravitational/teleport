import { render, screen } from 'design/utils/testing';
import Validation from 'shared/components/Validation';

import { Role } from './Role';
import { RoleProps } from './types';

const makeProps = (): RoleProps => {
  return {
    role: undefined,
    team: undefined,
    teamName: '',
    updateFields: () => {},
  };
};

test('hides custom team input for explicit fields', () => {
  const props = makeProps();
  render(
    <Validation>
      <Role {...props} />
    </Validation>
  );

  expect(screen.queryByLabelText('Team Name')).not.toBeInTheDocument();
});

test('shows custom team input', () => {
  const props = makeProps();
  props.team = 'OTHER';
  render(
    <Validation>
      <Role {...props} />
    </Validation>
  );

  expect(screen.getByLabelText('Team Name')).toBeInTheDocument();
});
