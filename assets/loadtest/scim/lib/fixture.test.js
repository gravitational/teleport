// node --test fixture.test.js

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import {
  groupName,
  groupSizesBetween,
  memberships,
  userName,
} from './fixture.js';

describe('groupSizesBetween', () => {
  it('keeps every size within [min, max]', () => {
    for (const size of groupSizesBetween(0, 1000, 200, 5000)) {
      assert.ok(size >= 200 && size <= 5000, `size ${size} out of range`);
    }
  });

  it('gives a group the same size whatever range it is computed in', () => {
    const all = groupSizesBetween(0, 20, 200, 5000);
    for (let g = 0; g < 20; g++) {
      assert.equal(groupSizesBetween(g, 1, 200, 5000)[0], all[g]);
    }
  });

  it('spreads sizes: m consecutive groups cover all m possible sizes', () => {
    const sizes = groupSizesBetween(0, 10, 1, 10);
    assert.deepEqual(
      [...sizes].sort((a, b) => a - b),
      [1, 2, 3, 4, 5, 6, 7, 8, 9, 10]
    );
  });

  it('gives every group the same size when min equals max', () => {
    assert.deepEqual(groupSizesBetween(5, 3, 50, 50), [50, 50, 50]);
  });
});

describe('memberships', () => {
  const groupStart = 3;
  const sizes = [2, 5, 3];
  const userPool = 10;
  const m = memberships(groupStart, sizes, userPool);
  const all = Array.from({ length: m.total }, (_, i) => m.at(i));

  it('totals the group sizes', () => {
    assert.equal(m.total, 10);
  });

  it('lists memberships group by group, sizes[k] for group k', () => {
    assert.deepEqual(
      all.map(x => x.group),
      [
        ...Array(2).fill(groupName(3)),
        ...Array(5).fill(groupName(4)),
        ...Array(3).fill(groupName(5)),
      ]
    );
  });

  it('never repeats a user within a group', () => {
    for (let k = 0; k < sizes.length; k++) {
      const users = all
        .filter(x => x.group === groupName(groupStart + k))
        .map(x => x.user);
      assert.equal(new Set(users).size, sizes[k]);
    }
  });

  it('draws users from the pool only', () => {
    const pool = new Set(
      Array.from({ length: userPool }, (_, i) => userName(i))
    );
    for (const { user } of all) {
      assert.ok(pool.has(user), `${user} not in pool`);
    }
  });

  it('starts group g at user (g * 7919) % userPool', () => {
    assert.equal(m.at(0).user, userName((3 * 7919) % userPool));
    assert.equal(m.at(2).user, userName((4 * 7919) % userPool));
    assert.equal(m.at(7).user, userName((5 * 7919) % userPool));
  });

  it('is deterministic', () => {
    const again = memberships(groupStart, sizes, userPool);
    assert.deepEqual(
      Array.from({ length: again.total }, (_, i) => again.at(i)),
      all
    );
  });
});
