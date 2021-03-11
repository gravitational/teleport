import { useState, useEffect } from 'react';
import useAttempt from 'shared/hooks/useAttemptNext';
import TeleportContextE from 'e-teleport/teleportContextE';
import { Invoice, formatCents } from 'e-teleport/services/cloud';

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
    periodText: `${unixDisplayDate(inv.periodEnd)}-${unixDisplayDate(
      inv.periodStart
    )}`,
    periodEndText: unixDisplayDate(inv.periodStart),
  }));
}

function unixDisplayDate(seconds: number) {
  return new Date(seconds * 1000).toLocaleDateString();
}
