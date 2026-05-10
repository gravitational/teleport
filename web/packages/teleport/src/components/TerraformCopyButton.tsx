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

import { useState, MouseEvent } from 'react';

import { Button } from 'design';
import { Check, Copy } from 'design/Icon';

export function TerraformCopyButton({
  onClick,
  disabled = false,
}: {
  onClick: (e: MouseEvent<HTMLButtonElement>) => void;
  disabled?: boolean;
}) {
  const [configCopied, setConfigCopied] = useState(false);

  const handleClick = (e: MouseEvent<HTMLButtonElement>) => {
    onClick(e);

    if (!e.defaultPrevented) {
      setConfigCopied(true);
      setTimeout(() => setConfigCopied(false), 1000);
    }
  };

  return (
    <Button
      fill="border"
      intent="primary"
      onClick={handleClick}
      gap={2}
      disabled={disabled}
    >
      {configCopied ? <Check size="small" /> : <Copy size="small" />}
      Copy Terraform Module
    </Button>
  );
}
