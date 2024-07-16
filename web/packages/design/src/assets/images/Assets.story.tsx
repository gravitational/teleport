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

import React from "react";

import Image from "design/Image";
import gravitationalLogo from "design/assets/images/gravitational-logo.svg";
import kubeLogo from "design/assets/images/kube-logo.svg";
import sampleLogoLong from "design/assets/images/sample-logo-long.svg";
import sampleLogoSquire from "design/assets/images/sample-logo-squire.svg";
import secKeyGraphic from "design/assets/images/sec-key-graphic.svg";
import teleportLogo from "design/assets/images/enterprise-light.svg";
import {TeleportLogoII} from "design/assets/images/TeleportLogoII";
import cloudCity from "design/assets/images/backgrounds/cloud-city.png"

export default {
    title: 'Design/Assets',
};

export const ImageSVG = () => (
    <div
        style={{
            display: 'grid',
            gridTemplateColumns: '100px 100px 100px',
            gridTemplateRows: '100px 100px 100px',
            columnGap: '15px',
            rowGap: '15px',
            alignItems: 'stretch',
        }}
    >
        <Image maxWidth="100px" maxHeight="100px" src={gravitationalLogo} />
        <Image maxWidth="100px" maxHeight="100px" src={kubeLogo} />
        <Image maxWidth="100px" maxHeight="100px" src={sampleLogoLong} />
        <Image maxWidth="100px" maxHeight="100px" src={sampleLogoSquire} />
        <Image maxWidth="100px" maxHeight="100px" src={secKeyGraphic} />
        <Image maxWidth="100px" maxHeight="100px" src={teleportLogo} />
    </div>
);

export const ReactSVG = () => (
    <TeleportLogoII/>
)

export const BackgroundsCloudCity = () => (
    <Image src={cloudCity} />
)
