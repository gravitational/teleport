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
