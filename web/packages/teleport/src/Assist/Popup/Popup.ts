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

import styled from 'styled-components';

import teleport from 'design/assets/images/icons/teleport.png';

export const PopupLogosSpacer = styled.div`
  padding: 0 8px;
`;

export const TeleportIcon = styled.div`
  background: url(${teleport}) no-repeat;
  width: 30px;
  height: 30px;
  background-size: contain;
  filter: invert(${p => (p.light ? '100%' : '0%')});
`;

export const PopupLogos = styled.div`
  display: flex;
  align-items: center;
`;

export const PopupFooter = styled.div`
  margin-top: 20px;
  display: flex;
  justify-content: space-between;
  align-items: center;
`;

export const Popup = styled.div`
  position: absolute;
  z-index: 100;
  top: 50px;
  right: -4px;
  background: ${({ theme }) => theme.colors.levels.popout};
  border-radius: 5px;
  width: 270px;
  font-size: 15px;
  padding: 20px 20px 15px;
  display: flex;
  flex-direction: column;

  &:after {
    content: '';
    position: absolute;
    width: 0;
    height: 0;
    border-style: solid;
    border-width: 0 10px 10px 10px;
    border-color: transparent transparent
      ${({ theme }) => theme.colors.levels.popout} transparent;
    right: 20px;
    top: -10px;
  }
`;

export const PopupTitle = styled.div`
  font-size: 18px;
  font-weight: bold;
  border-radius: 5px;
  margin-bottom: 15px;
`;

export const PopupTitleBackground = styled.span`
  background: linear-gradient(-45deg, #ee7752, #e73c7e);
  padding: 5px;
  border-radius: 5px;
  color: white;
`;

export const PopupButton = styled.div`
  cursor: pointer;
  display: inline-flex;
  border: 1px solid ${({ theme }) => theme.colors.text.slightlyMuted};
  color: ${({ theme }) => theme.colors.buttons.text};
  border-radius: 5px;
  padding: 8px 15px;

  &:hover {
    background: ${({ theme }) => theme.colors.buttons.border.hover};
  }
`;
