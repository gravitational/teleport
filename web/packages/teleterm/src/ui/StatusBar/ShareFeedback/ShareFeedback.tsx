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

import { useRef } from 'react';
import styled from 'styled-components';

import { ButtonText, Popover } from 'design';
import { ChatBubble } from 'design/Icon';

import { ShareFeedbackForm } from './ShareFeedbackForm';
import { useShareFeedback } from './useShareFeedback';

export function ShareFeedback() {
  const buttonRef = useRef<HTMLButtonElement>(null);
  const {
    submitFeedbackAttempt,
    formValues,
    hasBeenShareFeedbackOpened,
    isShareFeedbackOpened,
    submitFeedback,
    setFormValues,
    openShareFeedback,
    closeShareFeedback,
  } = useShareFeedback();

  return (
    <>
      <ButtonText
        css={`
          position: relative;
        `}
        setRef={buttonRef}
        title="Share feedback"
        size="small"
        onClick={openShareFeedback}
      >
        {!hasBeenShareFeedbackOpened && <NotOpenedYetIndicator />}
        <ChatBubble size="small" />
      </ButtonText>
      <Popover
        open={isShareFeedbackOpened}
        anchorEl={buttonRef.current}
        anchorOrigin={{ vertical: 'top', horizontal: 'right' }}
        transformOrigin={{ vertical: 'bottom', horizontal: 'right' }}
        growDirections="top-left"
        marginThreshold={8}
        onClose={closeShareFeedback}
        data-testid="share-feedback-container"
      >
        <ShareFeedbackForm
          formValues={formValues}
          submitFeedbackAttempt={submitFeedbackAttempt}
          onClose={closeShareFeedback}
          submitFeedback={submitFeedback}
          setFormValues={setFormValues}
        />
      </Popover>
    </>
  );
}

const NotOpenedYetIndicator = styled.div`
  position: absolute;
  top: 2px;
  left: 2px;
  height: 8px;
  width: 8px;
  background-color: ${props => props.theme.colors.info};
  border-radius: 50%;
  display: inline-block;
`;
