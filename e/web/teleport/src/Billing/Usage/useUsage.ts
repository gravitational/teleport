import { useState, useEffect } from 'react';
import useAttempt from 'shared/hooks/useAttemptNext';
import TeleportContextE from 'e-teleport/teleportContextE';
import { BillingCycle, formatCents } from 'e-teleport/services/cloud';

export default function useUsage(ctx: TeleportContextE) {
  const { attempt, run } = useAttempt('processing');
  const [cycles, setCycles] = useState<BillingCycle[]>([]);
  const [yearlyUsages, setYearlyUsages] = useState<YearlyUsage[]>([]);
  const [balance, setBalance] = useState('');
  const [productName, setProductName] = useState('');

  useEffect(() => {
    run(() => {
      return Promise.all([
        ctx.cloudService.fetchBillingInformation(),
        ctx.cloudService.fetchBillingCycles(),
      ]).then(response => {
        const [info, cycles] = response;
        const formatted = formatCents(info.account?.balance || 0);
        setBalance(formatted);
        setProductName(info.productName);

        if (cycles.length > 0) {
          setCycles(cycles);
          setYearlyUsages(getYearlyUsages(cycles));
        }
      });
    });
  }, []);

  return {
    attempt,
    balance,
    cycles,
    yearlyUsages,
    productName,
  };
}

// getYearlyUsages returns a sorted array (starting from most recent year),
// where each element represents a year, and each year contains a list of total
// costs of resources used (indexes represent month), and a list of resources
// that contain monthly count of resource used.
function getYearlyUsages(cycles: BillingCycle[]) {
  const cyclesMap = getYearlyCycles(cycles);
  let usages: YearlyUsage[] = [];

  Object.keys(cyclesMap)
    .sort((a, b) => Number(b) - Number(a))
    .forEach(year => {
      const cycles = cyclesMap[year];
      const items = getResourcesMonthlyItemCount(cycles);
      const totals = cycles.map(cycle => {
        // Convert to dollar amount.
        return cycle.totalAmount / 100;
      });

      usages = [
        ...usages,
        {
          year,
          totals,
          items,
        },
      ];
    });

  return usages;
}

// getYearlyCycles returns a map of years where each year contains
// a cycles list, sorted by cycles month. The indexes of this list,
// represent the cycles month (jan = 0, dec = 11).
//
// Note: When using built in array iterators (map, filter, foreach) that have callbacks,
// the cb's are not called for indexes that have never been set (empty slots in array).
// So these empty slots are ignored.
function getYearlyCycles(
  cycles: BillingCycle[]
): Record<string, BillingCycle[]> {
  const years = {};

  cycles.forEach(cycle => {
    const cycleDate = new Date(cycle.periodStart * 1000);

    // Starting period may begin at the end of previous cycle (last day of month).
    // Increment by one to get the correct starting month to get correct year.
    cycleDate.setDate(cycleDate.getDate() + 1);
    const year = cycleDate.getFullYear();

    const list = years[year];
    if (!list) {
      years[year] = new Array(12);
    }

    const month = cycleDate.getMonth();
    years[year][month] = cycle;
  });

  return years;
}

// getResourcesMonthlyItemCount returns a list of resources, with
// each resource containing monthly count of the resources used.

// Argument 'cycles' is expected to be formatted such that
// each indexes represent the month for the cycle.
function getResourcesMonthlyItemCount(cycles: BillingCycle[]) {
  const resources = {};

  cycles.forEach((cycle, index) => {
    const month = shortMonths[index];

    cycle.itemsList.forEach(item => {
      const kind = item.planResourceKind;
      if (!kind || kind === 'other') {
        return;
      }

      const name = item.planDescription;
      let resource = resources[name];
      if (!resource) {
        resource = { resource: name };
      }
      resource[month] = item.quantity;

      resources[name] = resource;
    });
  });

  const sortedByKind = Object.values(resources).sort(
    (a: MonthlyItem, b: MonthlyItem) => {
      if (a.resource < b.resource) {
        return -1;
      }
      if (a.resource > b.resource) {
        return 1;
      }
      return 0;
    }
  );

  return sortedByKind as unknown as MonthlyItem[];
}

const shortMonths = [
  'jan',
  'feb',
  'mar',
  'apr',
  'may',
  'jun',
  'jul',
  'aug',
  'sep',
  'oct',
  'nov',
  'dec',
] as const;

type Month = typeof shortMonths[number];
export type MonthlyItem = Partial<Record<Month, number>> & {
  resource: string;
};

export type State = ReturnType<typeof useUsage>;
export type YearlyUsage = {
  year: string;
  totals: number[];
  items: MonthlyItem[];
};
