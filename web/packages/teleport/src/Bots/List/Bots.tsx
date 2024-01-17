/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import React, { useEffect, useState } from 'react';

import { Alert, Box, ButtonPrimary, Indicator } from 'design';

import { useAttemptNext } from 'shared/hooks';

import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import useTeleport from 'teleport/useTeleport';
import { FlatBot } from 'teleport/Bots/types';
import { BotList } from 'teleport/Bots/List/BotList';

export function Bots() {
  const ctx = useTeleport();
  const [bots, setBots] = useState<FlatBot[]>();
  const { attempt, setAttempt, run } = useAttemptNext('processing');

  useEffect(() => {
    const signal = new AbortController();

    async function fetchBots(signal: AbortSignal) {
      try {
        const res = await ctx.botService.fetchBots({ signal });

        setBots(res.bots);
        setAttempt({ status: 'success' });
      } catch (err) {
        setAttempt({ status: 'failed', statusText: err.message });
      }
    }

    void fetchBots(signal.signal);
    return () => {
      signal.abort();
    };
  }, [run, ctx.botService, setAttempt]);

  return (
    <FeatureBox>
      <FeatureHeader>
        <FeatureHeaderTitle>Bots</FeatureHeaderTitle>
        <ButtonPrimary ml="auto" width="240px" disabled>
          Enroll New Bot
        </ButtonPrimary>
      </FeatureHeader>
      {attempt.status == 'processing' && (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}
      {attempt.status == 'failed' && (
        <Alert kind="danger" children={attempt.statusText} />
      )}
      {attempt.status == 'success' && <BotList bots={bots} />}
    </FeatureBox>
  );
}
