import { MemoryRouter } from 'react-router';

import {
  act,
  render,
  screen,
  testQueryClient,
  userEvent,
  waitFor,
  within,
} from 'design/utils/testing';
import { InfoGuidePanelProvider } from 'shared/components/SlidingSidePanel/InfoGuide';
import {
  ToastNotificationProvider,
  ToastNotifications,
} from 'shared/components/ToastNotification';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { ClientIpRestriction } from 'e-teleport/services/clientiprestrictions';
import { ContextProvider } from 'teleport';
import { ApiError } from 'teleport/services/api/parseError';
import { Access } from 'teleport/services/user';

import { ClientIpRestrictions, Countdown } from './ClientIpRestrictions';

const clusterId = 'cluster-123';

const allAccess: Access = {
  list: true,
  read: true,
  edit: true,
  create: true,
  remove: true,
};

const cir = (over: Partial<ClientIpRestriction> = {}): ClientIpRestriction => ({
  cidrs: ['10.0.0.0/8', '192.168.0.0/16'],
  mode: 'enforced',
  status: 'active',
  revision: 'rev-1',
  ...over,
});

function setup(
  over: {
    access?: Partial<Access>;
    resource?: ClientIpRestriction;
    /** What the write resolves to, when it should differ from what was fetched. */
    saved?: ClientIpRestriction;
    fetch?: jest.Mock;
    save?: jest.Mock;
  } = {}
) {
  const ctx = createTeleportContextE();
  ctx.storeUser.geClientIpRestrictionAccess = () => ({
    ...allAccess,
    ...over.access,
  });
  const fetched = over.resource ?? cir({ status: 'draft', mode: 'draft' });
  const fetch = over.fetch ?? jest.fn().mockResolvedValue(fetched);
  const save =
    over.save ??
    jest.fn().mockResolvedValue(over.saved ?? over.resource ?? cir());
  ctx.clientIpRestrictionsService.fetchClientIpRestriction = fetch;
  ctx.clientIpRestrictionsService.saveClientIpRestriction = save;

  render(
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <ToastNotificationProvider>
          <InfoGuidePanelProvider>
            <ClientIpRestrictions clusterId={clusterId} />
          </InfoGuidePanelProvider>
          {/* Mounted globally by the app; here so toasts can be asserted. */}
          <ToastNotifications />
        </ToastNotificationProvider>
      </ContextProvider>
    </MemoryRouter>
  );
  return { ctx, fetch, save, user: userEvent.setup() };
}

describe('ClientIpRestrictions', () => {
  beforeEach(async () => {
    await testQueryClient.resetQueries();
    testQueryClient.clear();
    jest.clearAllMocks();
  });

  test('renders nothing without read access', () => {
    setup({ access: { read: false } });
    expect(screen.queryByText(/IP Allowlist/i)).not.toBeInTheDocument();
  });

  test('draft: shows the draft banner and draft actions', async () => {
    setup({
      resource: cir({ status: 'draft', mode: 'draft' }),
    });
    expect(await screen.findByText(/^Draft$/i)).toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: /Enforce/i })
    ).toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: /Test Run/i })
    ).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /^Edit$/i })).toBeInTheDocument();
  });

  test('apply from draft writes enforced with no expiry', async () => {
    const { save, user } = setup({
      resource: cir({ status: 'draft', mode: 'draft' }),
    });
    await user.click(await screen.findByRole('button', { name: /Enforce/i }));
    await waitFor(() =>
      expect(save).toHaveBeenCalledWith(clusterId, {
        cidrs: ['10.0.0.0/8', '192.168.0.0/16'],
        mode: 'enforced',
        revision: 'rev-1',
      })
    );
  });

  test('starting a test run from draft writes enforced with an expiry', async () => {
    const { save, user } = setup({
      resource: cir({ status: 'draft', mode: 'draft' }),
    });
    await user.click(await screen.findByRole('button', { name: /Test Run/i }));
    await waitFor(() => expect(save).toHaveBeenCalled());
    const [, req] = save.mock.calls[0];
    expect(req.mode).toBe('enforced');
    expect(req.revision).toBe('rev-1');
    expect(new Date(req.expires).getTime()).toBeGreaterThan(Date.now());
  });

  test('active: shows deactivate, which confirms via dialog before reverting', async () => {
    const { save, user } = setup({
      resource: cir({ status: 'active', mode: 'enforced' }),
    });
    expect(await screen.findByText(/^Active$/i)).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: /Deactivate/i }));
    // dialog appears; nothing written yet
    const dialog = await screen.findByRole('dialog');
    expect(
      within(dialog).getByText(/Deactivate IP allowlist\?/i)
    ).toBeInTheDocument();
    expect(save).not.toHaveBeenCalled();

    // confirm in the dialog
    await user.click(
      within(dialog).getByRole('button', { name: /^Deactivate$/i })
    );
    await waitFor(() =>
      expect(save).toHaveBeenCalledWith(clusterId, {
        cidrs: ['10.0.0.0/8', '192.168.0.0/16'],
        mode: 'draft',
        revision: 'rev-1',
      })
    );
  });

  test('pending: cancel writes a draft, which is the way back out of a rollout', async () => {
    const { save, user } = setup({
      resource: cir({ status: 'pending', mode: 'enforced' }),
    });
    expect(await screen.findByText(/^Pending$/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /^Edit$/i })).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: /^Cancel$/i }));
    await waitFor(() =>
      expect(save).toHaveBeenCalledWith(clusterId, {
        cidrs: ['10.0.0.0/8', '192.168.0.0/16'],
        mode: 'draft',
        revision: 'rev-1',
      })
    );
  });

  test('pending: a test run puts a deadline on the rollout in flight', async () => {
    const { save, user } = setup({
      resource: cir({ status: 'pending', mode: 'enforced' }),
    });
    await user.click(await screen.findByRole('button', { name: /Test Run/i }));
    await waitFor(() => expect(save).toHaveBeenCalled());
    const [, req] = save.mock.calls[0];
    // Same list, still enforced: the only change is the deadline.
    expect(req.cidrs).toEqual(['10.0.0.0/8', '192.168.0.0/16']);
    expect(req.mode).toBe('enforced');
    expect(req.revision).toBe('rev-1');
    expect(new Date(req.expires).getTime()).toBeGreaterThan(Date.now());
  });

  test('a draft write still rolling out reads as returning to draft', async () => {
    // The teardown is in flight, which the server cannot report as draft yet.
    setup({
      resource: cir({ status: 'pending', mode: 'draft' }),
    });
    expect(await screen.findByText(/Returning to draft/i)).toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: /^Enforce$/i })
    ).toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: /Test Run/i })
    ).toBeInTheDocument();
  });

  test('a test run rolling out counts down while the rules go out', async () => {
    setup({
      resource: cir({
        status: 'pending',
        mode: 'enforced',
        expires: new Date(Date.now() + 5 * 60 * 1000).toISOString(),
      }),
    });
    expect(await screen.findByText(/Test run pending/i)).toBeInTheDocument();
    // The window drains during the rollout, so it is shown from the start.
    expect(await screen.findByText(/[45]:\d\d remaining/)).toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: /Stop test/i })
    ).toBeInTheDocument();
  });

  test('a lapsed test run stays enforced, so confirm is still offered', async () => {
    // The deadline passed; the rules stay programmed until Cloud removes them.
    setup({
      resource: cir({
        status: 'active',
        mode: 'enforced',
        expires: new Date(Date.now() - 60_000).toISOString(),
      }),
    });
    expect(await screen.findByText(/Test run expired/i)).toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: /Confirm/i })
    ).toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: /Stop test/i })
    ).toBeInTheDocument();
    // Nothing left to count down to.
    expect(screen.queryByText(/remaining/)).not.toBeInTheDocument();
  });

  test('unknown: offers only refresh, never a write', async () => {
    // A status this client does not recognise. Enforcement may well be live, so
    // saving would stop it silently; Edit is deliberately absent.
    setup({ resource: cir({ status: 'unknown', mode: 'enforced' }) });
    expect(await screen.findByText(/could not determine/i)).toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: /Refresh/i })
    ).toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: /^Edit$/i })
    ).not.toBeInTheDocument();
  });

  test('the status block stays visible while editing', async () => {
    const { user } = setup({
      resource: cir({ status: 'active', mode: 'enforced' }),
    });
    await user.click(await screen.findByRole('button', { name: /^Edit$/i }));
    expect(screen.getByText(/^Active$/i)).toBeInTheDocument();
  });

  test('active test run shows a countdown; confirm clears the expiry', async () => {
    const expires = new Date(Date.now() + 5 * 60 * 1000).toISOString();
    const { save, user } = setup({
      resource: cir({ status: 'active', mode: 'enforced', expires }),
    });
    expect(await screen.findByText(/Test run active/i)).toBeInTheDocument();
    // The countdown itself, not just the title: 4:5x or 5:00 depending on timing.
    expect(await screen.findByText(/[45]:\d\d remaining/)).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: /Confirm/i }));
    await waitFor(() =>
      expect(save).toHaveBeenCalledWith(clusterId, {
        cidrs: ['10.0.0.0/8', '192.168.0.0/16'],
        mode: 'enforced',
        revision: 'rev-1',
      })
    );
  });

  test('expired: shows the expired banner and offers apply without the stale expiry', async () => {
    // A lapsed test run: mode still enforced, the elapsed expiry still present.
    const { save, user } = setup({
      resource: cir({
        status: 'expired',
        mode: 'enforced',
        expires: new Date(Date.now() - 60 * 60 * 1000).toISOString(),
      }),
    });
    expect(await screen.findByText(/^Expired$/i)).toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: /Test Run/i })
    ).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: /^Enforce$/i }));
    await waitFor(() =>
      expect(save).toHaveBeenCalledWith(clusterId, {
        cidrs: ['10.0.0.0/8', '192.168.0.0/16'],
        mode: 'enforced',
        revision: 'rev-1',
      })
    );
  });

  test('editing a draft and saving writes a draft with the edited list', async () => {
    const { save, user } = setup({
      resource: cir({ status: 'draft', mode: 'draft' }),
    });
    await user.click(await screen.findByRole('button', { name: /^Edit$/i }));

    await user.clear(screen.getByRole('textbox'));
    await user.type(screen.getByRole('textbox'), '10.0.0.0/8\n172.16.0.0/12');
    await user.click(screen.getByRole('button', { name: /^Save$/i }));

    await waitFor(() =>
      expect(save).toHaveBeenCalledWith(clusterId, {
        cidrs: ['10.0.0.0/8', '172.16.0.0/12'],
        mode: 'draft',
        revision: 'rev-1',
      })
    );
  });

  test('editing during a test run saves a draft and offers a new test run', async () => {
    const { save, user } = setup({
      resource: cir({
        status: 'active',
        mode: 'enforced',
        expires: new Date(Date.now() + 5 * 60 * 1000).toISOString(),
      }),
    });
    await user.click(await screen.findByRole('button', { name: /^Edit$/i }));
    await user.clear(screen.getByRole('textbox'));
    await user.type(screen.getByRole('textbox'), '10.0.0.0/8\n172.16.0.0/12');
    await user.click(screen.getByRole('button', { name: /^Save$/i }));

    // The edited list is not what was under test, so the test run ends.
    await waitFor(() =>
      expect(save).toHaveBeenCalledWith(clusterId, {
        cidrs: ['10.0.0.0/8', '172.16.0.0/12'],
        mode: 'draft',
        revision: 'rev-1',
      })
    );
    expect(
      await screen.findByText(/Start a new test run/i)
    ).toBeInTheDocument();
  });

  test('saving an unchanged list writes nothing', async () => {
    const { save, user } = setup({
      resource: cir({ status: 'draft', mode: 'draft' }),
    });
    await user.click(await screen.findByRole('button', { name: /^Edit$/i }));
    await user.click(screen.getByRole('button', { name: /^Save$/i }));

    expect(save).not.toHaveBeenCalled();
  });

  test('saving an unchanged list still writes when it stops a rollout', async () => {
    // From pending, saving is not a no-op: it returns the allowlist to draft.
    const { save, user } = setup({
      resource: cir({ status: 'pending', mode: 'enforced' }),
    });
    await user.click(await screen.findByRole('button', { name: /^Edit$/i }));
    await user.click(screen.getByRole('button', { name: /^Save$/i }));

    await waitFor(() =>
      expect(save).toHaveBeenCalledWith(clusterId, {
        cidrs: ['10.0.0.0/8', '192.168.0.0/16'],
        mode: 'draft',
        revision: 'rev-1',
      })
    );
  });

  test('an empty draft cannot be enforced or test run', async () => {
    // An empty allowlist allows all traffic, but the server reports it as active,
    // so the panel would claim the cluster is restricted while it is wide open.
    setup({ resource: cir({ status: 'draft', mode: 'draft', cidrs: [] }) });
    expect(await screen.findByText(/^Draft$/i)).toBeInTheDocument();

    expect(screen.getByRole('button', { name: /^Enforce$/i })).toBeDisabled();
    expect(screen.getByRole('button', { name: /Test Run/i })).toBeDisabled();
  });

  test('emptying the list while enforced cannot be saved', async () => {
    const { save, user } = setup({
      resource: cir({ status: 'active', mode: 'enforced' }),
    });
    await user.click(await screen.findByRole('button', { name: /^Edit$/i }));
    await user.clear(screen.getByRole('textbox'));

    // Deactivate is the way out, and it asks for confirmation first.
    expect(screen.getByRole('button', { name: /^Save$/i })).toBeDisabled();
    expect(save).not.toHaveBeenCalled();
  });

  test('a failed write keeps the editor open with what was typed', async () => {
    const save = jest.fn().mockRejectedValue(new Error('revision conflict'));
    const { user } = setup({
      resource: cir({ status: 'draft', mode: 'draft' }),
      save,
    });
    await user.click(await screen.findByRole('button', { name: /^Edit$/i }));
    await user.clear(screen.getByRole('textbox'));
    await user.type(screen.getByRole('textbox'), '10.0.0.0/8\n172.16.0.0/12');
    await user.click(screen.getByRole('button', { name: /^Save$/i }));

    await waitFor(() => expect(save).toHaveBeenCalled());
    // Still editing, with the typed list intact.
    expect(screen.getByRole('button', { name: /^Save$/i })).toBeInTheDocument();
    expect(screen.getByRole('textbox')).toHaveValue(
      '10.0.0.0/8\n172.16.0.0/12'
    );
    expect(
      await screen.findByText(/Failed to update the IP allowlist/i)
    ).toBeInTheDocument();
  });

  test('a stale revision is reported in our words, not the server text', async () => {
    // Expected whenever someone else writes while the panel is open, so the raw
    // "created|modified|deleted" wording must not reach the customer.
    const save = jest.fn().mockRejectedValue(
      new ApiError({
        message:
          'resource revision does not match, it may have been concurrently created|modified|deleted; please work from the latest state',
        response: new Response(null, { status: 412 }),
      })
    );
    const { user } = setup({
      resource: cir({ status: 'draft', mode: 'draft' }),
      save,
    });

    await user.click(await screen.findByRole('button', { name: /Enforce/i }));
    expect(
      await screen.findByText(/Someone else changed the allowlist/i)
    ).toBeInTheDocument();
    expect(screen.queryByText(/concurrently created/i)).not.toBeInTheDocument();
  });

  test('a failed deactivate stays open and shows why, and does not linger', async () => {
    // The mutation keeps its error until the next write, so reopening the dialog
    // must not greet the customer with a failure they already dealt with.
    const save = jest.fn().mockRejectedValue(new Error('revision conflict'));
    const { user } = setup({
      resource: cir({ status: 'active', mode: 'enforced' }),
      save,
    });

    await user.click(
      await screen.findByRole('button', { name: /Deactivate/i })
    );
    const dialog = await screen.findByRole('dialog');
    await user.click(
      within(dialog).getByRole('button', { name: /^Deactivate$/i })
    );
    expect(await screen.findByText(/revision conflict/i)).toBeInTheDocument();

    // Close it, then open it again: the old failure must be gone.
    await user.click(
      within(await screen.findByRole('dialog')).getByRole('button', {
        name: /^Cancel$/i,
      })
    );
    await user.click(screen.getByRole('button', { name: /Deactivate/i }));
    expect(
      within(await screen.findByRole('dialog')).queryByText(
        /revision conflict/i
      )
    ).not.toBeInTheDocument();
  });

  test('a poll landing mid-edit leaves the buffer alone', async () => {
    const fetch = jest
      .fn()
      .mockResolvedValueOnce(cir({ status: 'draft', mode: 'draft' }))
      .mockResolvedValue(
        cir({ status: 'draft', mode: 'draft', cidrs: ['172.16.0.0/12'] })
      );
    const { user } = setup({ fetch });

    await user.click(await screen.findByRole('button', { name: /^Edit$/i }));
    await user.clear(screen.getByRole('textbox'));
    await user.type(screen.getByRole('textbox'), '10.0.0.0/8');

    await testQueryClient.refetchQueries();

    // The server's list changed underneath; the editor keeps what was typed.
    expect(screen.getByRole('textbox')).toHaveValue('10.0.0.0/8');
  });

  test('a save carries the revision the edit began from, not a fresher polled one', async () => {
    // Someone else wrote rev-2 while the editor was open. The buffer is still
    // based on rev-1, so the save must send rev-1 and let the server report the
    // conflict, not adopt rev-2 and silently replace the other admin's list.
    const fetch = jest
      .fn()
      .mockResolvedValueOnce(
        cir({ status: 'draft', mode: 'draft', revision: 'rev-1' })
      )
      .mockResolvedValue(
        cir({
          status: 'draft',
          mode: 'draft',
          cidrs: ['172.16.0.0/12'],
          revision: 'rev-2',
        })
      );
    const { save, user } = setup({ fetch });

    await user.click(await screen.findByRole('button', { name: /^Edit$/i }));
    await user.clear(screen.getByRole('textbox'));
    await user.type(screen.getByRole('textbox'), '10.0.0.0/8');

    await testQueryClient.refetchQueries();

    await user.click(screen.getByRole('button', { name: /^Save$/i }));
    await waitFor(() =>
      expect(save).toHaveBeenCalledWith(clusterId, {
        cidrs: ['10.0.0.0/8'],
        mode: 'draft',
        revision: 'rev-1',
      })
    );
  });

  test('a tenant with no allowlist gets an editor, and its first write creates the resource', async () => {
    // What Cloud returns before anything is configured.
    const { save, user } = setup({
      resource: {
        cidrs: [],
        mode: '',
        status: '',
        revision: '00000000-0000-0000-0000-000000000000',
      },
    });
    expect(await screen.findByText(/Not configured/i)).toBeInTheDocument();
    expect(screen.queryByText(/could not determine/i)).not.toBeInTheDocument();

    // Saving an empty editor has nothing to create, so it writes nothing.
    await user.click(screen.getByRole('button', { name: /^Edit$/i }));
    await user.click(screen.getByRole('button', { name: /^Save$/i }));
    expect(save).not.toHaveBeenCalled();

    await user.click(screen.getByRole('button', { name: /^Edit$/i }));
    await user.type(screen.getByRole('textbox'), '10.0.0.0/8');
    await user.click(screen.getByRole('button', { name: /^Save$/i }));

    // The nil revision must not be sent, or the write is a guarded update.
    await waitFor(() =>
      expect(save).toHaveBeenCalledWith(clusterId, {
        cidrs: ['10.0.0.0/8'],
        mode: 'draft',
        revision: '',
      })
    );
  });

  test('a first-create save conflicts when someone else created the allowlist mid-edit', async () => {
    // The edit began before the resource existed, so its base revision is empty
    // and the write would be an unguarded upsert — the server cannot catch this
    // conflict, so the client must refuse rather than silently replace the
    // allowlist the other admin just created (possibly enforced).
    const fetch = jest
      .fn()
      .mockResolvedValueOnce({
        cidrs: [],
        mode: '',
        status: '',
        revision: '00000000-0000-0000-0000-000000000000',
      })
      .mockResolvedValue(
        cir({ status: 'active', mode: 'enforced', revision: 'rev-2' })
      );
    const { save, user } = setup({ fetch });

    expect(await screen.findByText(/Not configured/i)).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: /^Edit$/i }));
    await user.type(screen.getByRole('textbox'), '10.0.0.0/8');

    await testQueryClient.refetchQueries();

    await user.click(screen.getByRole('button', { name: /^Save$/i }));
    expect(
      await screen.findByText(/Someone else changed the allowlist/i)
    ).toBeInTheDocument();
    expect(save).not.toHaveBeenCalled();
    // Still editing, with the typed list intact.
    expect(screen.getByRole('textbox')).toHaveValue('10.0.0.0/8');
  });

  test('the panel follows the resource the write returns', async () => {
    // Cancel during a rollout: the panel has to follow the write, not stay Pending.
    const fetch = jest
      .fn()
      .mockResolvedValueOnce(cir({ status: 'pending', mode: 'enforced' }))
      .mockResolvedValue(cir({ status: 'pending', mode: 'draft' }));
    const { user } = setup({
      fetch,
      saved: cir({ status: 'pending', mode: 'draft' }),
    });
    expect(await screen.findByText(/^Pending$/i)).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: /^Cancel$/i }));
    expect(await screen.findByText(/Returning to draft/i)).toBeInTheDocument();
  });

  test('read-only user sees refresh but no write actions', async () => {
    setup({
      access: { edit: false, create: false },
      resource: cir({ status: 'active', mode: 'enforced' }),
    });
    expect(await screen.findByText(/^Active$/i)).toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: /Refresh/i })
    ).toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: /Deactivate/i })
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: /^Edit$/i })
    ).not.toBeInTheDocument();
  });

  test('shows an error with retry when the fetch fails', async () => {
    const fetch = jest.fn().mockRejectedValue(new Error('boom'));
    setup({ fetch });
    expect(await screen.findByText(/boom/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Retry/i })).toBeInTheDocument();
  });
});

const at = (offsetMs: number) => new Date(Date.now() + offsetMs).toISOString();

const advance = (ms: number) => act(() => jest.advanceTimersByTime(ms));

describe('Countdown', () => {
  beforeEach(() => jest.useFakeTimers());
  afterEach(() => jest.useRealTimers());

  test('formats the remaining time as M:SS under an hour', () => {
    render(<Countdown expires={at(65_000)} />);
    expect(screen.getByText('1:05')).toBeInTheDocument();

    advance(6_000);
    expect(screen.getByText('0:59')).toBeInTheDocument();
  });

  // The UI only starts 30-minute test runs, but tctl and Terraform accept any
  // deadline, so the panel must render long ones readably (not "39457:21").
  test('formats long deadlines with the two largest units', () => {
    const { unmount } = render(
      <Countdown expires={at(2 * 3_600_000 + 5 * 60_000)} />
    );
    expect(screen.getByText('2h 5m')).toBeInTheDocument();
    unmount();

    render(<Countdown expires={at(27 * 86_400_000 + 13 * 3_600_000)} />);
    expect(screen.getByText('27d 13h')).toBeInTheDocument();
  });

  test('drops back to M:SS as a long deadline drains below an hour', () => {
    render(<Countdown expires={at(3_600_000 + 30_000)} />);
    expect(screen.getByText('1h 0m')).toBeInTheDocument();

    advance(31_000);
    expect(screen.getByText('59:59')).toBeInTheDocument();
  });

  test('clamps at zero rather than counting into negatives', () => {
    render(<Countdown expires={at(-60_000)} />);
    expect(screen.getByText('0:00')).toBeInTheDocument();
  });

  test('fires onExpire once when the deadline passes, not on every tick', () => {
    const onExpire = jest.fn();
    render(<Countdown expires={at(2_000)} onExpire={onExpire} />);
    expect(onExpire).not.toHaveBeenCalled();

    advance(3_000);
    expect(onExpire).toHaveBeenCalledTimes(1);

    advance(10_000);
    expect(onExpire).toHaveBeenCalledTimes(1);
  });

  test('a new expiry remounts, restarting the countdown and re-arming onExpire', () => {
    const onExpire = jest.fn();
    const first = at(2_000);
    const { rerender } = render(
      <Countdown key={first} expires={first} onExpire={onExpire} />
    );
    advance(3_000);
    expect(onExpire).toHaveBeenCalledTimes(1);

    // The panel keys this on `expires`, so a second test run is a fresh mount.
    const second = at(2_000);
    rerender(<Countdown key={second} expires={second} onExpire={onExpire} />);
    expect(screen.getByText('0:02')).toBeInTheDocument();

    advance(3_000);
    expect(onExpire).toHaveBeenCalledTimes(2);
  });
});
