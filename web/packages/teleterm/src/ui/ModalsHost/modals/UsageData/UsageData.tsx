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

import { ButtonIcon, ButtonPrimary, ButtonSecondary, H2, Link } from 'design';
import DialogConfirmation, {
  DialogContent,
  DialogFooter,
  DialogHeader,
} from 'design/DialogConfirmation';
import { Cross } from 'design/Icon';
import { P } from 'design/Text/Text';

export function UsageData(props: {
  onCancel(): void;
  onAllow(): void;
  onDecline(): void;
  hidden?: boolean;
}) {
  return (
    <DialogConfirmation
      open={!props.hidden}
      keepInDOMAfterClose
      onClose={props.onCancel}
      dialogCss={() => ({
        maxWidth: '400px',
        width: '100%',
      })}
    >
      <form
        onSubmit={e => {
          e.preventDefault();
          props.onAllow();
        }}
      >
        <DialogHeader
          justifyContent="space-between"
          mb={0}
          alignItems="baseline"
        >
          <H2 mb={4}>Anonymous usage data</H2>
          <ButtonIcon
            type="button"
            onClick={props.onCancel}
            color="text.slightlyMuted"
          >
            <Cross size="medium" />
          </ButtonIcon>
        </DialogHeader>
        <DialogContent mb={4}>
          <P>
            Do you agree to Teleport Connect collecting anonymized usage data?
            This will help us improve the product.
          </P>
          <P>
            To learn more, see{' '}
            <Link
              href="https://goteleport.com/docs/faq/#teleport-connect"
              target="_blank"
            >
              our documentation
            </Link>
            .
          </P>
        </DialogContent>
        <DialogFooter>
          <ButtonPrimary autoFocus mr={3} type="submit">
            Allow
          </ButtonPrimary>
          <ButtonSecondary type="button" onClick={props.onDecline}>
            Decline
          </ButtonSecondary>
        </DialogFooter>
      </form>
    </DialogConfirmation>
  );
}
