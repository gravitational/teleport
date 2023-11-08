/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React, { useState, useRef, useCallback } from 'react';

import ButtonIcon from 'design/ButtonIcon';
import { Check, Copy } from 'design/Icon';
import copyToClipboard from 'design/utils/copyToClipboard';

import { HoverTooltip } from 'shared/components/ToolTip';

export function CopyButton({ name }: { name: string }) {
  const copySuccess = 'Copied!';
  const copyDefault = 'Click to copy';
  const copyAnchorEl = useRef(null);
  const [copiedText, setCopiedText] = useState(copyDefault);

  const handleCopy = useCallback(() => {
    setCopiedText(copySuccess);
    copyToClipboard(name);
    // Change to default text after 1 second
    setTimeout(() => {
      setCopiedText(copyDefault);
    }, 1000);
  }, [name]);

  return (
    <HoverTooltip tipContent={<>{copiedText}</>}>
      <ButtonIcon setRef={copyAnchorEl} size={0} ml={1} onClick={handleCopy}>
        {copiedText === copySuccess ? (
          <Check size="small" />
        ) : (
          <Copy size="small" />
        )}
      </ButtonIcon>
    </HoverTooltip>
  );
}
