import { useState, useEffect } from 'react';
import useAttempt from 'shared/hooks/useAttemptNext';
import TeleportContextE from 'e-teleport/teleportContextE';
import { BillingCycle, formatCents } from 'e-teleport/services/cloud';

export default function useUsage(ctx: TeleportContextE) {
  const { attempt, run } = useAttempt('processing');
  const [cycles, setCycles] = useState<BillingCycle[]>([]);
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
        setCycles(cycles);
      });
    });
  }, []);

  return {
    attempt,
    balance,
    cycles,
    productName,
  };
}

export type State = ReturnType<typeof useUsage>;
