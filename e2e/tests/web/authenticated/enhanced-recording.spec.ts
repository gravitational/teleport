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

import { skipEnhancedRecording } from '@gravitational/e2e/helpers/env';
import { expect, test } from '@gravitational/e2e/helpers/test';

// In proxy recording mode the node discards its own session.end so the forwarding node's copy is
// the only one in the audit log, and the forwarding node never runs BPF. It therefore has to be
// told by the node that Enhanced Session Recording was active.
test.describe('enhanced session recording in proxy recording mode', () => {
  test.use({
    // Nothing here is browser-specific, and repeating it would also repeat the Teleport restart the
    // recording mode below needs.
    browsers: ['chromium'],
    teleport: {
      config: {
        auth_service: {
          session_recording: 'proxy',
        },
      },
    },
  });

  test.describe('on a node running it', () => {
    test.use({ fixtures: ['ssh-node-bpf'] });

    test.skip(
      skipEnhancedRecording,
      "the docker daemon's kernel cannot run enhanced session recording"
    );

    test('session.end reports enhanced recording', async ({
      unifiedResourcesPage,
      auditLogPage,
    }) => {
      await unifiedResourcesPage.goto();

      const terminal = await unifiedResourcesPage.connect(
        'docker-node-bpf',
        'root'
      );
      await terminal.waitForReady();

      const sessionID = await terminal.sessionID();

      // Run a command so BPF has something to capture, which also proves enhanced recording is
      // actually running rather than just configured.
      await terminal.exec('ls /');
      await terminal.waitForText('bin');

      await terminal.exit();

      await auditLogPage.goto();
      await auditLogPage.search(sessionID);

      // BPF emits this one directly from the node, so its presence means enhanced recording really
      // ran and the flag on session.end is not just being set optimistically.
      await auditLogPage.waitForEvent('session.command', sessionID);

      const sessionEnd = await auditLogPage.waitForEvent(
        'session.end',
        sessionID
      );
      expect(
        await auditLogPage.eventField(sessionEnd, 'enhanced_recording')
      ).toBe('true');
    });
  });

  test.describe('on a node without it', () => {
    test.use({ fixtures: ['ssh-node'] });

    test('session.end reports no enhanced recording', async ({
      unifiedResourcesPage,
      auditLogPage,
    }) => {
      await unifiedResourcesPage.goto();

      const terminal = await unifiedResourcesPage.connect(
        'docker-node',
        'root'
      );
      await terminal.waitForReady();

      const sessionID = await terminal.sessionID();

      // The same command as the node running it, so a node wrongly running BPF leaves the same trace.
      await terminal.exec('ls /');
      await terminal.waitForText('bin');

      await terminal.exit();

      await auditLogPage.goto();
      await auditLogPage.search(sessionID);

      const sessionEnd = await auditLogPage.waitForEvent(
        'session.end',
        sessionID
      );
      expect(
        await auditLogPage.eventField(sessionEnd, 'enhanced_recording')
      ).toBe('false');

      // session.end is written as the session tears down, so anything BPF would have emitted for it
      // is already in the log.
      await auditLogPage.expectNoEvent('session.command', sessionID);
    });
  });
});
