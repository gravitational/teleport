import { NodeToken } from 'teleport/services/nodes';
import { createNodeBashCommand } from './useAddNode';

describe('correct node bash command', () => {
  const now = new Date();

  test.each`
    token            | hours  | expires      | cmd
    ${'some-token'}  | ${1.1} | ${'1 hour'}  | ${'sudo bash -c "$(curl -fsSL http://localhost/scripts/some-token/install-node.sh)"'}
    ${'other-token'} | ${2.1} | ${'2 hours'} | ${'sudo bash -c "$(curl -fsSL http://localhost/scripts/other-token/install-node.sh)"'}
    ${'some-token'}  | ${25}  | ${'1 day'}   | ${'sudo bash -c "$(curl -fsSL http://localhost/scripts/some-token/install-node.sh)"'}
  `(
    'test bash command with: $token expiring in $hours',
    ({ token, hours, expires, cmd }) => {
      const node: NodeToken = {
        expiry: addHours(now, hours),
        id: token,
      };
      const bashCommand = createNodeBashCommand(node);
      expect(bashCommand.expires).toBe(expires);
      expect(bashCommand.text).toBe(cmd);
    }
  );
});

function addHours(date: Date, hours: number): Date {
  return new Date(date.getTime() + hours * 60 * 60 * 1000);
}
