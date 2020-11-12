/**
 * Copyright 2020 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import DefaultApps from './Apps';
import { createMemoryHistory } from 'history';
import { Router } from 'react-router';
import makeAcl from 'teleport/services/user/makeAcl';
import { apps } from './fixtures';
import { ContextProvider, Context } from 'teleport';

export default {
  title: 'Teleport/Apps',
};

export const Loaded = () => {
  const ctx = new Context();
  const acl = makeAcl(sample.acl);

  ctx.isEnterprise = true;
  ctx.storeUser.setState({ acl });
  ctx.appService.fetchApps = () => Promise.resolve(apps);
  return render(ctx);
};

export const Empty = () => {
  const ctx = new Context();
  const acl = makeAcl(sample.acl);

  ctx.isEnterprise = true;
  ctx.storeUser.setState({ acl });
  ctx.storeUser.state.acl.tokens.create = true;
  ctx.appService.fetchApps = () => Promise.resolve([]);
  return render(ctx);
};

export const EmptyReadOnly = () => {
  const ctx = new Context();
  const acl = makeAcl(sample.acl);

  ctx.isEnterprise = true;
  ctx.storeUser.setState({ acl });
  ctx.storeUser.state.acl.tokens.create = false;
  ctx.appService.fetchApps = () => Promise.resolve([]);
  return render(ctx);
};

export const Processing = () => {
  const ctx = new Context();
  const acl = makeAcl(sample.acl);

  ctx.isEnterprise = true;
  ctx.storeUser.setState({ acl });
  ctx.appService.fetchApps = () => new Promise(() => null);
  return render(ctx);
};

export const Failed = () => {
  const ctx = new Context();
  const acl = makeAcl(sample.acl);

  ctx.isEnterprise = true;
  ctx.storeUser.setState({ acl });
  ctx.appService.fetchApps = () =>
    Promise.reject(new Error('some error message'));
  return render(ctx);
};

function render(ctx) {
  const history = createMemoryHistory({
    initialEntries: ['/web/cluster/localhost/audit/events'],
    initialIndex: 0,
  });

  return (
    <ContextProvider ctx={ctx}>
      <Router history={history}>
        <DefaultApps />
      </Router>
    </ContextProvider>
  );
}

const sample = {
  acl: {
    tokens: {
      create: true,
    },
    apps: {
      list: true,
      create: true,
      remove: true,
      edit: true,
      read: true,
    },
  },
};
