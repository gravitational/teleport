import { createBashCommand } from './useAddNode';

import type { JoinToken } from 'teleport/services/joinToken';

describe('correct bash command', () => {
  test.each`
    token            | method     | cmd
    ${'some-token'}  | ${'token'} | ${'sudo bash -c "$(curl -fsSL http://localhost/scripts/some-token/install-node.sh)"'}
    ${'other-token'} | ${'token'} | ${'sudo bash -c "$(curl -fsSL http://localhost/scripts/other-token/install-node.sh)"'}
    ${'some-token'}  | ${'iam'}   | ${'sudo bash -c "$(curl -fsSL http://localhost/scripts/some-token/install-node.sh?method=iam)"'}
  `(
    'test bash command with: $token expiring in $hours',
    ({ token, method, cmd }) => {
      const node: JoinToken = {
        id: token,
        expiry: null,
      };
      const bashCommand = createBashCommand(node.id, method);
      expect(bashCommand).toBe(cmd);
    }
  );
});
