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
import { Link } from 'react-router-dom';

import { Alert, Box, ButtonPrimary, Indicator } from 'design';

import { useAttemptNext } from 'shared/hooks';

import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import { FlatBot } from 'teleport/Bots/types';
import { BotList } from 'teleport/Bots/List/BotList';
import { fetchBots } from 'teleport/services/bot/bot';
import cfg from 'teleport/config';

export function Bots() {
  const [bots, setBots] = useState<FlatBot[]>();
  const { attempt, run } = useAttemptNext('processing');

  useEffect(() => {
    const signal = new AbortController();

    async function init(signal: AbortSignal) {
      const res = await fetchBots({ signal });
      setBots(res.bots);
    }

    run(() => init(signal.signal));
    return () => {
      signal.abort();
    };
  }, [run]);

  return (
    <FeatureBox>
      <FeatureHeader>
        <FeatureHeaderTitle>Bots</FeatureHeaderTitle>
        <ButtonPrimary
          ml="auto"
          width="240px"
          as={Link}
          to={cfg.getBotsNewRoute()}
        >
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
