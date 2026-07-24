/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import { MemoryRouter } from 'react-router';

import { render, screen } from 'design/utils/testing';

import { ContextProvider } from 'teleport';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { App } from 'teleport/services/apps';

import {
  llmAnthropicApp,
  llmBedrockApp,
  llmOpenAIApp,
  llmOpenAIBedrockApp,
} from './fixtures';
import { LLMAppConnectDialog } from './LLMAppConnectDialog';

function renderDialog(app: App) {
  const ctx = createTeleportContext();
  return render(
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <LLMAppConnectDialog app={app} onClose={() => {}} />
      </ContextProvider>
    </MemoryRouter>
  );
}

const anthropicBaseUrl = 'export ANTHROPIC_BASE_URL=http://127.0.0.1:3000';
const openaiBaseUrl = 'export OPENAI_BASE_URL=http://127.0.0.1:3000/v1';
const codexCommand = 'codex -c openai_base_url=http://127.0.0.1:3000/v1';
const bedrockEnv = 'export CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS=1';
const codexBedrockCommand =
  'codex -c model_providers.amazon-bedrock.base_url=http://127.0.0.1:3000 ' +
  '-c model_providers.amazon-bedrock.auth.command=cat ' +
  '-c model_provider=amazon-bedrock';

test('anthropic endpoint shows Claude instructions only', () => {
  renderDialog(llmAnthropicApp);

  expect(
    screen.getByText('tsh proxy app anthropic --port 3000')
  ).toBeInTheDocument();
  expect(screen.getByText(anthropicBaseUrl)).toBeInTheDocument();
  expect(screen.queryByText(openaiBaseUrl)).not.toBeInTheDocument();
  expect(screen.queryByText(bedrockEnv)).not.toBeInTheDocument();
});

test('bedrock endpoint adds the disable-experimental-betas setup', () => {
  renderDialog(llmBedrockApp);

  expect(screen.getByText(anthropicBaseUrl)).toBeInTheDocument();
  expect(screen.getByText(bedrockEnv)).toBeInTheDocument();
  expect(screen.queryByText(openaiBaseUrl)).not.toBeInTheDocument();
});

test('openai endpoint shows Codex instructions only', () => {
  renderDialog(llmOpenAIApp);

  expect(
    screen.getByText('tsh proxy app openai --port 3000')
  ).toBeInTheDocument();
  expect(screen.getByText(openaiBaseUrl)).toBeInTheDocument();
  expect(screen.getByText(codexCommand)).toBeInTheDocument();
  expect(screen.queryByText(anthropicBaseUrl)).not.toBeInTheDocument();
});

test('openai endpoint on Bedrock uses the Codex Bedrock model provider', () => {
  renderDialog(llmOpenAIBedrockApp);

  expect(screen.getByText(codexBedrockCommand)).toBeInTheDocument();
  // The Bedrock model provider carries the address and auth inline, so the
  // OPENAI_* environment variables and the plain Codex command are not shown.
  expect(screen.queryByText(openaiBaseUrl)).not.toBeInTheDocument();
  expect(screen.queryByText(codexCommand)).not.toBeInTheDocument();
});
