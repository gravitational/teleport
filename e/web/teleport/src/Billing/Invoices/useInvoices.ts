import { useState, useEffect } from 'react';
import useAttempt from 'shared/hooks/useAttemptNext';
import TeleportContextE from 'e-teleport/teleportContextE';
import { Invoice, formatCents } from 'e-teleport/services/cloud';
import { displayUnixDate } from 'shared/services/loc';

export default function useInvoices(ctx: TeleportContextE) {
  const { attempt, run } = useAttempt('processing');
  const [invoices, setInvoices] = useState<ListItem[]>([]);

  useEffect(() => {
    run(() =>
      ctx.cloudService.fetchInvoices().then(res => {
        setInvoices(makeListItems(res));
      })
    );
  }, []);

  return {
    attempt,
    invoices,
  };
}

export type State = ReturnType<typeof useInvoices>;

export interface ListItem extends Invoice {
  periodText: string;
  periodEndText: string;
  amountDueText: string;
  amountPaidText: string;
}

export function makeListItems(invoices: Invoice[]): ListItem[] {
  invoices = invoices || [];
  return invoices.map(inv => ({
    ...inv,
    amountDueText: formatCents(inv.amountDue),
    amountPaidText: formatCents(inv.amountPaid),
    periodText: `${displayUnixDate(inv.periodEnd)} - ${displayUnixDate(
      inv.periodStart
    )}`,
    periodEndText: displayUnixDate(inv.periodStart),
  }));
}
