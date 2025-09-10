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

import { useCallback, useEffect, useState } from 'react';
import { Link, useHistory } from 'react-router-dom';

import { Alert, Box, Button, Indicator } from 'design';
import { HoverTooltip } from 'design/Tooltip';
import { InfoGuideButton } from 'shared/components/SlidingSidePanel/InfoGuide';
import { useAttemptNext } from 'shared/hooks';

import { BotList } from 'teleport/Bots/List/BotList';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import cfg from 'teleport/config';
import { fetchBots } from 'teleport/services/bot/bot';
import { FlatBot } from 'teleport/services/bot/types';
import useTeleport from 'teleport/useTeleport';

import { InfoGuide } from '../InfoGuide';
import { EmptyState } from './EmptyState/EmptyState';

export function Bots() {
  const ctx = useTeleport();
  const history = useHistory();
  const flags = ctx.getFeatureFlags();
  const hasAddBotPermissions = flags.addBots;
  const canListBots = flags.listBots;

  const [bots, setBots] = useState<FlatBot[]>([]);
  const [selectedBot, setSelectedBot] = useState<FlatBot | null>();
  const { attempt: fetchAttempt, run: fetchRun } = useAttemptNext(
    canListBots ? 'processing' : 'success'
  );

  useEffect(() => {
    const signal = new AbortController();
    const flags = ctx.getFeatureFlags();

    async function bots(signal: AbortSignal) {
      return await fetchBots(signal, flags);
    }

    if (canListBots) {
      fetchRun(() =>
        bots(signal.signal).then(botRes => {
          setBots(botRes.bots);
        })
      );
    }
    return () => {
      signal.abort();
    };
  }, [ctx, fetchRun, canListBots]);

  function onDelete() {
    setBots(bots.filter(bot => bot.name !== selectedBot.name));
    onClose();
  }

  function onEdit(updated: FlatBot) {
    const updatedList = bots.map((item: FlatBot): FlatBot => {
      if (item.name !== selectedBot?.name) {
        return item;
      }
      return {
        ...item,
        ...updated,
      };
    });

    setBots(updatedList);
    onClose();
  }

  function onClose() {
    setSelectedBot(null);
  }

  const handleSelect = useCallback(
    (item: FlatBot) => {
      history.push(cfg.getBotDetailsRoute(item.name));
    },
    [history]
  );

  if (fetchAttempt.status === 'processing') {
    return (
      <FeatureBox>
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      </FeatureBox>
    );
  }

  if (fetchAttempt.status === 'success' && bots.length === 0) {
    return (
      <FeatureBox>
        {!canListBots && (
          <Alert kind="info" mt={4}>
            You do not have permission to access Bots. Missing role permissions:{' '}
            <code>bot.list</code>
          </Alert>
        )}
        <EmptyState />
      </FeatureBox>
    );
  }

  return (
    <FeatureBox>
      <FeatureHeader>
        <FeatureHeaderTitle>Bots</FeatureHeaderTitle>
        <Box ml="auto">
          <InfoGuideButton config={{ guide: <InfoGuide /> }}>
            <HoverTooltip
              tipContent={
                hasAddBotPermissions
                  ? ''
                  : `Insufficient permissions. Reach out to your Teleport administrator
    to request bot creation permissions.`
              }
            >
              <Button
                intent="primary"
                fill={
                  fetchAttempt.status === 'success' && bots.length === 0
                    ? 'filled'
                    : 'border'
                }
                ml="auto"
                width="240px"
                as={Link}
                to={cfg.getBotsNewRoute()}
                disabled={!hasAddBotPermissions}
              >
                Enroll New Bot
              </Button>
            </HoverTooltip>
          </InfoGuideButton>
        </Box>
      </FeatureHeader>
      {fetchAttempt.status == 'failed' && (
        <Alert kind="danger">{fetchAttempt.statusText}</Alert>
      )}
      {fetchAttempt.status == 'success' && (
        <BotList
          bots={bots}
          disabledEdit={!flags.roles || !flags.editBots}
          disabledDelete={!flags.removeBots}
          onClose={onClose}
          onDelete={onDelete}
          onEdit={onEdit}
          onSelect={handleSelect}
          selectedBot={selectedBot}
          setSelectedBot={setSelectedBot}
        />
      )}
    </FeatureBox>
  );
}
