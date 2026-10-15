const USER_PREFIX = 'k6-user-';
const USER_DOMAIN = 'example.com';
const GROUP_PREFIX = 'k6-group-';
/**
 * SMOKE_GROUP_NAME is the title of the `scim` Access List
 * that the smoke test links. `make smoke` creates and deletes that list.
 * Keep it in sync to SMOKE_GROUP in the Makefile and to the name and title in
 * smoke.yaml.
 */
export const SMOKE_GROUP_NAME = 'k6-group-0-smoke';

/**
 * SMOKE_USER_NAME is the only user that the smoke test creates, adds to
 * SMOKE_GROUP_NAME, removes and deletes.
 */
export const SMOKE_USER_NAME = 'k6-user-0-smoke@example.com';

/**
 * userName generates loadtest username.
 * @param {number} i
 * @returns {string} of k6-user-{i}@example.com format
 */
export function userName(i) {
  return `${USER_PREFIX}${i}@${USER_DOMAIN}`;
}

/**
 * groupName generates loadtest group name.
 * @param {number} i
 * @returns {string} of k6-group-{i} format.
 */
export function groupName(i) {
  return `${GROUP_PREFIX}${i}`;
}

/**
 * 7919 is 1000th prime number, large enough to be used for deducing deterministic group size
 * and the first member of each group, as (group * PRIME) % range.
 *
 * Edge case: if the range is MEMBERS_MAX - MEMBERS_MIN + 1,  e.g., MEMBERS_MIN=1 and
 * MEMBERS_MAX=7919 gives every group MEMBERS_MIN members.
 */
const PRIME = 7919;

/**
 * groupSizesBetween returns count sizes for groups groupStart .. groupStart+count-1, each in [min, max].
 * @param {number} groupStart
 * @param {number} count
 * @param {number} min
 * @param {number} max
 * @returns {number} size.
 */
export function groupSizesBetween(groupStart, count, min, max) {
  const range = max - min + 1; // number of possible sizes
  const sizes = [];
  for (let i = 0; i < count; i++) {
    const group = groupStart + i;
    sizes.push(min + ((group * PRIME) % range));
  }
  return sizes;
}

/**
 * memberships lists the memberships of groups groupStart .. groupStart+sizes.length-1,
 * group by group. at(i) computes the i-th member on demand.
 */
export function memberships(groupStart, sizes, userPool) {
  // offsets[k]: index of group k's first membership.
  const offsets = [];
  let total = 0;
  for (const size of sizes) {
    offsets.push(total);
    total += size;
  }

  // at returns the index-th membership as { group, user }.
  function at(index) {
    const k = lastAtOrBelow(offsets, index);
    const g = groupStart + k;
    const i = index - offsets[k];
    return { group: groupName(g), user: userName((g * PRIME + i) % userPool) };
  }

  return { total, at };
}

/**
 * lastAtOrBelow returns the position of the last value <= target in an ascending array.
 */
function lastAtOrBelow(sorted, target) {
  let lo = 0;
  let hi = sorted.length - 1;
  while (lo < hi) {
    const mid = Math.ceil((lo + hi) / 2);
    if (sorted[mid] <= target) {
      lo = mid;
    } else {
      hi = mid - 1;
    }
  }
  return lo;
}
