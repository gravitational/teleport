import { ClientIpRestriction } from 'e-teleport/services/clientiprestrictions';

import {
  deriveUiState,
  getRemainingMs,
  isSettling,
  pollIntervalFor,
  TEST_RUN_DURATION_MS,
  testRunPayload,
  writeRevision,
} from './utils';

const cir = (over: Partial<ClientIpRestriction>): ClientIpRestriction => ({
  cidrs: ['10.0.0.0/8'],
  mode: 'enforced',
  status: 'active',
  revision: 'rev-1',
  ...over,
});

describe('deriveUiState', () => {
  test('returns unknown when the resource is missing', () => {
    expect(deriveUiState(undefined)).toBe('unknown');
  });

  test('maps draft regardless of expiry', () => {
    expect(deriveUiState(cir({ status: 'draft' }))).toBe('draft');
    expect(
      deriveUiState(cir({ status: 'draft', expires: '2999-01-01T00:00:00Z' }))
    ).toBe('draft');
  });

  test('distinguishes pending from a test run applying', () => {
    expect(deriveUiState(cir({ status: 'pending' }))).toBe('pending');
    expect(
      deriveUiState(cir({ status: 'pending', expires: '2999-01-01T00:00:00Z' }))
    ).toBe('testRunApplying');
  });

  test('reads a pending draft write as returning to draft', () => {
    // The mode is what tells the two directions of a pending write apart.
    expect(deriveUiState(cir({ status: 'pending', mode: 'draft' }))).toBe(
      'returningToDraft'
    );
  });

  test('distinguishes active from a test run active', () => {
    expect(deriveUiState(cir({ status: 'active' }))).toBe('active');
    expect(
      deriveUiState(cir({ status: 'active', expires: '2999-01-01T00:00:00Z' }))
    ).toBe('testRunActive');
  });

  test('maps a status outside the four to unknown, whatever the mode', () => {
    // Statuses were migrated, so an unrecognisable one means something is wrong,
    // not a resource that predates the field. The mode no longer changes this.
    expect(deriveUiState(cir({ status: '', mode: 'enforced' }))).toBe(
      'unknown'
    );
    expect(deriveUiState(cir({ status: 'unknown', mode: 'draft' }))).toBe(
      'unknown'
    );
    expect(deriveUiState(cir({ status: 'unknown', mode: '' }))).toBe('unknown');
    // Nothing saved at all needs an editor, not an error banner.
    expect(deriveUiState(cir({ status: '', mode: '', cidrs: [] }))).toBe(
      'notConfigured'
    );
    expect(deriveUiState(cir({ status: 'unknown', mode: '', cidrs: [] }))).toBe(
      'notConfigured'
    );
  });

  test('maps expired to its own state, not to a live test run', () => {
    expect(
      deriveUiState(
        cir({
          status: 'expired',
          mode: 'enforced',
          expires: '2020-01-01T00:00:00Z',
        })
      )
    ).toBe('expired');
  });

  test('reports a test run whose deadline passed as ending until the server confirms', () => {
    const now = Date.UTC(2026, 0, 1, 0, 0, 0);
    const past = new Date(now - 60_000).toISOString();
    // The rules stay programmed until the Cloud side removes them.
    expect(deriveUiState(cir({ status: 'active', expires: past }), now)).toBe(
      'testRunEnding'
    );
    expect(deriveUiState(cir({ status: 'pending', expires: past }), now)).toBe(
      'testRunEnding'
    );
  });
});

describe('writeRevision', () => {
  test('passes a real revision through', () => {
    expect(writeRevision('rev-1')).toBe('rev-1');
  });

  test('drops the nil UUID and the empty case', () => {
    expect(writeRevision('00000000-0000-0000-0000-000000000000')).toBe('');
    expect(writeRevision(undefined)).toBe('');
    expect(writeRevision('')).toBe('');
  });
});

describe('poll cadence', () => {
  // Sentinels, so the choice is verified independently of the real values.
  const intervals = { settlingMs: 1, stableMs: 2 };

  test('picks the settling cadence while settling', () => {
    for (const s of [
      'pending',
      'returningToDraft',
      'testRunApplying',
      'testRunActive',
      'testRunEnding',
    ] as const) {
      expect(isSettling(s)).toBe(true);
      expect(pollIntervalFor(s, intervals)).toBe(intervals.settlingMs);
    }
  });

  test('picks the stable cadence when stable', () => {
    for (const s of ['draft', 'active', 'expired', 'unknown'] as const) {
      expect(isSettling(s)).toBe(false);
      expect(pollIntervalFor(s, intervals)).toBe(intervals.stableMs);
    }
  });
});

describe('getRemainingMs', () => {
  const now = Date.UTC(2026, 0, 1, 0, 0, 0);

  test('returns 0 without an expiry', () => {
    expect(getRemainingMs(undefined, now)).toBe(0);
  });

  test('returns 0 for a past or invalid expiry', () => {
    expect(getRemainingMs(new Date(now - 1000).toISOString(), now)).toBe(0);
    expect(getRemainingMs('not-a-date', now)).toBe(0);
  });

  test('reads the expiry as UTC whatever the browser timezone is', () => {
    // Same instant three ways; the last is what JavaScript reads as local.
    expect(getRemainingMs('2026-01-01T00:01:30Z', now)).toBe(90_000);
    expect(getRemainingMs('2026-01-01T02:01:30+02:00', now)).toBe(90_000);
    expect(getRemainingMs('2026-01-01T00:01:30', now)).toBe(90_000);
  });
});

describe('testRunPayload', () => {
  const now = Date.UTC(2026, 0, 1, 0, 0, 0);

  test('enforces with an expiry TEST_RUN_DURATION_MS ahead', () => {
    const payload = testRunPayload(['10.0.0.0/8'], 'rev-2', now);
    expect(payload.mode).toBe('enforced');
    expect(payload.revision).toBe('rev-2');
    expect(payload.expires).toBe(
      new Date(now + TEST_RUN_DURATION_MS).toISOString()
    );
  });
});
