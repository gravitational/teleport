import renderHook, { act } from 'design/utils/renderHook';
import TeleportContextE from 'e-teleport/teleportContextE';
import { BillingCycle } from 'e-teleport/services/cloud';
import useUsage from './useUsage';

test('formatting of cycles and its item list in yearlyUsages', async () => {
  const ctx = new TeleportContextE();
  ctx.cloudService.fetchBillingInformation = () => Promise.resolve({} as any);
  ctx.cloudService.fetchBillingCycles = () => Promise.resolve(cycles);

  let hook;
  await act(async () => {
    hook = renderHook(() => useUsage(ctx));
  });

  expect(hook.current.yearlyUsages).toHaveLength(2);

  // Test first in array is the most recent year (2021)
  const list1 = hook.current.yearlyUsages[0];
  expect(list1.year).toBe('2021');
  expect(list1.items).toEqual([
    { resource: 'Application Access', jan: 0 },
    { resource: 'Database Access', jan: 1, apr: 3333 },
    { resource: 'Kubernetes Access', jan: 350 },
    { resource: 'Server Access', jan: 5 },
  ]);

  // Test totals indexes match month of cycle
  //  and only non-empty slots in array are processed
  let setCyclesTotalAmts = {};
  list1.totals.forEach((cycle, index) => {
    setCyclesTotalAmts[index] = cycle;
  });
  expect(setCyclesTotalAmts).toEqual({
    0: 160.94,
    3: 250,
  });
  expect(list1.totals).toHaveLength(12);

  // Test older list (2020)
  const list2 = hook.current.yearlyUsages[1];
  expect(list2.year).toBe('2020');
  expect(list2.items).toEqual([{ resource: 'Database Access', dec: 1 }]);

  setCyclesTotalAmts = {};
  list2.totals.forEach((cycle, index) => {
    setCyclesTotalAmts[index] = cycle;
  });
  expect(setCyclesTotalAmts).toEqual({
    11: 160.94,
  });
  expect(list2.totals).toHaveLength(12);

  // Test empty cycles array results in empty array.
  ctx.cloudService.fetchBillingCycles = () => Promise.resolve([]);
  await act(async () => {
    hook = renderHook(() => useUsage(ctx));
  });

  expect(hook.current.yearlyUsages).toEqual([]);
});

const cycleApr2021: BillingCycle = {
  cycleId: 57,
  state: 'started',
  totalAmount: 25000,
  periodStart: 1617235200, // Wed Mar 31 2021
  periodEnd: 1619827200, // Fri Apr 30 2021
  itemsList: [
    {
      cycleItemId: 'ed632646-7266-4803-8a07-d9e82f001d2c',
      quantity: 3333,
      amount: 0,
      planScheme: 'tiered',
      planResourceKind: 'database',
      planDescription: 'Database Access',
    },
  ],
};

const cycleJan2021: BillingCycle = {
  cycleId: 0,
  state: 'ended',
  totalAmount: 16094,
  periodStart: 1609486693, // dec 31 2020
  periodEnd: 1612165093, // jan 31 2021,
  itemsList: [
    {
      cycleItemId: '54e97268-7a41-45b5-9395-88db34eee37f',
      quantity: 1,
      amount: 5000,
      planScheme: 'tiered',
      planResourceKind: 'database',
      planDescription: 'Database Access',
    },
    {
      cycleItemId: '9b864173-17fb-4660-a6bc-2144325d10a6',
      quantity: 5,
      amount: 499999,
      planScheme: 'tiered',
      planResourceKind: 'server',
      planDescription: 'Server Access',
    },
    {
      cycleItemId: 'ef8233ad-2d36-46c0-bf99-b77de75f337e',
      quantity: 0,
      amount: 0,
      planScheme: 'tiered',
      planResourceKind: 'application',
      planDescription: 'Application Access',
    },
    {
      cycleItemId: '861f1c68-4bfd-45d5-a7df-7cab105ce6f6',
      quantity: 350,
      amount: 25555,
      planScheme: 'tiered',
      planResourceKind: 'kube_cluster',
      planDescription: 'Kubernetes Access',
    },
  ],
};

const cycleDec2020: BillingCycle = {
  cycleId: 0,
  state: 'ended',
  totalAmount: 16094,
  periodStart: 1606799286, // nov 30 2020
  periodEnd: 1609486693, // dec 31 2020
  itemsList: [
    {
      cycleItemId: '54e97268-7a41-45b5-9395-88db34eee37f',
      quantity: 1,
      amount: 5000,
      planScheme: 'tiered',
      planResourceKind: 'database',
      planDescription: 'Database Access',
    },
  ],
};

const cycles = [cycleApr2021, cycleDec2020, cycleJan2021];
