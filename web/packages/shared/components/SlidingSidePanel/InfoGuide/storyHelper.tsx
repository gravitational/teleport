/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import Box from 'design/Box';

import { InfoParagraph, InfoTitle, ReferenceLinks } from './InfoGuide';

export const LongGuideContent = ({ withPadding = false }) => (
  <Box px={withPadding ? 3 : 0}>
    <InfoTitle>Each title is wrapped with InfoTitle component</InfoTitle>
    <InfoParagraph>
      Each paragraphs are wrapped with InfoParagraph component.
    </InfoParagraph>
    <InfoTitle>InfoTitle Two</InfoTitle>
    <InfoParagraph>
      2 Lorem ipsum dolor sit, amet consectetur adipisicing elit. Commodi
      corrupti voluptates aliquam eligendi placeat harum rerum ipsam. Corrupti
      architecto laudantium, libero perspiciatis officia doloremque est aliquam,
      eius qui tenetur.
    </InfoParagraph>
    <InfoTitle>InfoTitle Three</InfoTitle>
    <InfoParagraph>
      3 Lorem ipsum dolor sit, amet consectetur adipisicing elit. Commodi
      corrupti voluptates aliquam eligendi placeat harum rerum ipsam. Corrupti
      architecto laudantium, libero perspiciatis officia doloremque est aliquam,
      eius qui tenetur.
    </InfoParagraph>
    <InfoTitle>InfoTitle Four</InfoTitle>
    <InfoParagraph>
      4 Lorem ipsum dolor sit, amet consectetur adipisicing elit. Commodi
      corrupti voluptates aliquam eligendi placeat harum rerum ipsam. Corrupti
      architecto laudantium, libero perspiciatis officia doloremque est aliquam,
      eius qui tenetur.
    </InfoParagraph>
    <InfoTitle>InfoTitle Five</InfoTitle>
    <InfoParagraph>
      5 Lorem ipsum dolor sit, amet consectetur adipisicing elit. Commodi
      corrupti voluptates aliquam eligendi placeat harum rerum ipsam. Corrupti
      architecto laudantium, libero perspiciatis officia doloremque est aliquam,
      eius qui tenetur.
    </InfoParagraph>
    <ReferenceLinks
      links={[
        { title: 'Some Link 1', href: 'link1' },
        { title: 'Some Link 2', href: 'link2' },
        { title: 'Some Link 3', href: 'link3' },
        { title: 'Some Link 4', href: 'link4' },
        { title: 'Some Link 5', href: 'link5' },
      ]}
    />
  </Box>
);
