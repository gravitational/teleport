/*
Copyright 2019-2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';
import styled from 'styled-components';
import { space, fontSize, width, color } from 'styled-system';
import '../assets/icomoon/style.css';

const Icon = styled.span`
  display: inline-block;
  transition: color 0.3s;
  ${space} ${width} ${color} ${fontSize}
`;

Icon.displayName = `Icon`;
Icon.defaultProps = {
  color: 'light',
};

function makeFontIcon(name, iconClassName) {
  const iconClass = `icon ${iconClassName}`;
  return function ({ className = '', ...rest }) {
    const classes = `${iconClass} ${className}`;
    return <Icon className={classes} {...rest} />;
  };
}

export const Add = makeFontIcon('Add', 'icon-add');
export const AddUsers = makeFontIcon('AddUsers', 'icon-users-plus');
export const AmazonAws = makeFontIcon('AmazonAws', 'icon-amazonaws');
export const Amex = makeFontIcon('Amex', 'icon-cc-amex');
export const Apartment = makeFontIcon('Apartment', 'icon-apartment');
export const AppInstalled = makeFontIcon('AppInstalled', 'icon-app-installed');
export const Apple = makeFontIcon('Apple', 'icon-apple');
export const AppRollback = makeFontIcon('AppRollback', 'icon-app-rollback');
export const Archive = makeFontIcon('Archive', 'icon-archive2');
export const ArrowDown = makeFontIcon('ArrowDown', 'icon-chevron-down');
export const ArrowForward = makeFontIcon('ArrowForward', 'icon-arrow_forward');
export const ArrowBack = makeFontIcon('ArrowBack', 'icon-arrow_back');
export const ArrowLeft = makeFontIcon('ArrowLeft', 'icon-chevron-left');
export const ArrowRight = makeFontIcon('ArrowRight', 'icon-chevron-right');
export const ArrowsVertical = makeFontIcon(
  'ArrowsVertical',
  'icon-chevrons-expand-vertical'
);
export const ArrowUp = makeFontIcon('ArrowUp', 'icon-chevron-up');
export const AlarmRing = makeFontIcon('AlarmRing', 'icon-alarm-ringing');
export const BagDollar = makeFontIcon('BagDollar', 'icon-bag-dollar');
export const BitBucket = makeFontIcon('Bitbucket', 'icon-bitbucket');
export const OpenBox = makeFontIcon('OpenBox', 'icon-box');
export const Bubble = makeFontIcon('Bubble', 'icon-bubble');
export const Camera = makeFontIcon('Camera', 'icon-camera');
export const CardView = makeFontIcon('CardView', 'icon-th-large');
export const CardViewSmall = makeFontIcon('CardViewSmall', 'icon-th');
export const CaretLeft = makeFontIcon('CaretLeft', 'icon-caret-left');
export const CaretRight = makeFontIcon('CaretRight', 'icon-caret-right');
export const CarrotDown = makeFontIcon('CarrotDown', 'icon-caret-down');
export const CarrotLeft = makeFontIcon('CarrotLeft', 'icon-caret-left');
export const CarrotRight = makeFontIcon('CarrotRight', 'icon-caret-right');
export const CarrotSort = makeFontIcon('CarrotSort', 'icon-sort');
export const CarrotUp = makeFontIcon('CarrotUp', 'icon-caret-up');
export const Cash = makeFontIcon('Cash', 'icon-cash-dollar');
export const ChatBubble = makeFontIcon(
  'ChatBubble',
  'icon-chat_bubble_outline'
);
export const Chart = makeFontIcon('Chart', 'icon-chart-bars');
export const Check = makeFontIcon('Check', 'icon-check');
export const ChevronCircleDown = makeFontIcon(
  'ChevronCircleDown',
  'icon-chevron-down-circle'
);
export const ChevronCircleLeft = makeFontIcon(
  'ChevronCircleLeft',
  'icon-chevron-left-circle'
);
export const ChevronCircleRight = makeFontIcon(
  'ChevronCircleRight',
  'icon-chevron-right-circle'
);
export const ChevronCircleUp = makeFontIcon(
  'ChevronCircleUp',
  'icon-chevron-up-circle'
);
export const CircleArrowLeft = makeFontIcon(
  'CircleArrowLeft',
  'icon-arrow-left-circle'
);
export const CircleArrowRight = makeFontIcon(
  'CircleArrowRight',
  'icon-arrow-right-circle'
);
export const CircleCheck = makeFontIcon('CircleCheck', 'icon-checkmark-circle');
export const CircleCross = makeFontIcon('CircleCross', 'icon-cross-circle');
export const CirclePause = makeFontIcon('CirclePause', 'icon-pause-circle');
export const CirclePlay = makeFontIcon('CirclePlay', 'icon-play-circle');
export const CircleStop = makeFontIcon('CircleStop', 'icon-stop-circle');
export const Cli = makeFontIcon('Cli', 'icon-terminal');
export const Clipboard = makeFontIcon('Clipboard', 'icon-clipboard-text');
export const ClipboardUser = makeFontIcon(
  'ClipboardUser',
  'icon-clipboard-user'
);
export const Clock = makeFontIcon('Clock', 'icon-clock3');
export const Close = makeFontIcon('Close', 'icon-close');
export const Cloud = makeFontIcon('Cloud', 'icon-cloud');
export const CloudSync = makeFontIcon('CloudSync', 'icon-cloud-sync');
export const Cluster = makeFontIcon('Cluster', 'icon-site-map');
export const Clusters = makeFontIcon('Clusters', 'icon-icons2');
export const ClusterAdded = makeFontIcon('ClusterAdded', 'icon-cluster-added');
export const ClusterAuth = makeFontIcon('ClusterAuth', 'icon-cluster-auth');
export const Code = makeFontIcon('Code', 'icon-code');
export const Cog = makeFontIcon('Cog', 'icon-cog');
export const Config = makeFontIcon('Config', 'icon-config');
export const Contract = makeFontIcon('Contract', 'icon-frame-contract');
export const Copy = makeFontIcon('Copy', 'icon-copy');
export const CreditCard = makeFontIcon('CreditCard', 'icon-credit-card1');
export const CreditCardAlt = makeFontIcon(
  'CreditCardAlt',
  'icon-credit-card-alt'
);
export const CreditCardAlt2 = makeFontIcon(
  'CreditCardAlt2',
  'icon-credit-card'
);
export const Cross = makeFontIcon('Cross', 'icon-cross');
export const Database = makeFontIcon('Database', 'icon-database');
export const Desktop = makeFontIcon('Desktop', 'icon-desktop');
export const Discover = makeFontIcon('Discover', 'icon-cc-discover');
export const Download = makeFontIcon('Download', 'icon-get_app');
export const Earth = makeFontIcon('Earth', 'icon-earth');
export const Edit = makeFontIcon('Edit', 'icon-pencil4');
export const Ellipsis = makeFontIcon('Ellipsis', 'icon-ellipsis');
export const EmailSolid = makeFontIcon('EmailSolid', 'icon-email-solid');
export const EnvelopeOpen = makeFontIcon('EnvelopeOpen', 'icon-envelope-open');
export const Equalizer = makeFontIcon('Equalizer', 'icon-equalizer');
export const EqualizerVertical = makeFontIcon(
  'EqualizerVertical',
  'icon-equalizer1'
);
export const ExitRight = makeFontIcon('ExitRight', 'icon-exit-right');
export const Expand = makeFontIcon('Expand', 'icon-frame-expand');
export const Facebook = makeFontIcon('Facebook', 'icon-facebook');
export const FacebookSquare = makeFontIcon('FacebookSquare', 'icon-facebook2');
export const FileCode = makeFontIcon('Youtube', 'icon-file-code');
export const FolderPlus = makeFontIcon('FolderPlus', 'icon-folder-plus');
export const FolderShared = makeFontIcon('FolderShared', 'icon-folder-shared');
export const ForwarderAdded = makeFontIcon(
  'ForwarderAdded',
  'icon-add-fowarder'
);
export const Github = makeFontIcon('Github', 'icon-github');
export const Google = makeFontIcon('Google', 'icon-google-plus');
export const Graph = makeFontIcon('Graph', 'icon-graph');
export const Hashtag = makeFontIcon('Hashtag', 'icon-hashtag');
export const Home = makeFontIcon('Home', 'icon-home3');
export const Info = makeFontIcon('Info', 'icon-info_outline');
export const InfoFilled = makeFontIcon('Info', 'icon-info');
export const Key = makeFontIcon('Key', 'icon-key');
export const Keypair = makeFontIcon('Keypair', 'icon-keypair');
export const Kubernetes = makeFontIcon('Kubernetes', 'icon-kubernetes');
export const Label = makeFontIcon('Label', 'icon-label');
export const Lan = makeFontIcon('Lan', 'icon-lan');
export const LanAlt = makeFontIcon('LanAlt', 'icon-lan2');
export const Layers = makeFontIcon('Layers', 'icon-layers');
export const Layers1 = makeFontIcon('Layers1', 'icon-layers1');
export const License = makeFontIcon('License', 'icon-license2');
export const Link = makeFontIcon('Link', 'icon-link');
export const Linkedin = makeFontIcon('Linkedin', 'icon-linkedin');
export const Linux = makeFontIcon('Linux', 'icon-linux');
export const List = makeFontIcon('List', 'icon-list');
export const ListThin = makeFontIcon('ListThin', 'icon-list1');
export const ListAddCheck = makeFontIcon(
  'ListAddCheck',
  'icon-playlist_add_check'
);
export const ListBullet = makeFontIcon('ListBullet', 'icon-list4');
export const ListCheck = makeFontIcon('ListCheck', 'icon-list3');
export const ListView = makeFontIcon('ListView', 'icon-th-list');
export const LocalPlay = makeFontIcon('LocalPlay', 'icon-local_play');
export const Lock = makeFontIcon('Lock', 'icon-lock');
export const Magnifier = makeFontIcon('Magnifier', 'icon-magnifier');
export const MasterCard = makeFontIcon('MasterCard', 'icon-cc-mastercard');
export const Memory = makeFontIcon('Memory', 'icon-memory');
export const MoreHoriz = makeFontIcon('MoreHoriz', 'icon-more_horiz');
export const MoreVert = makeFontIcon('MoreVert', 'icon-more_vert');
export const Mute = makeFontIcon('Mute', 'icon-mute');
export const NewTab = makeFontIcon('NewTab', 'icon-new-tab');
export const NoteAdded = makeFontIcon('NoteAdded', 'icon-note_add');
export const NotificationsActive = makeFontIcon(
  'NotificationsActive',
  'icon-notifications_active'
);
export const OpenID = makeFontIcon('OpenID', 'icon-openid');
export const PaperPlane = makeFontIcon('PaperPlane', 'icon-paper-plane');
export const Paypal = makeFontIcon('Paypal', 'icon-cc-paypal');
export const Pencil = makeFontIcon('Pencil', 'icon-pencil');
export const Person = makeFontIcon('Person', 'icon-person');
export const PersonAdd = makeFontIcon('PersonAdd', 'icon-person_add');
export const PhonelinkErase = makeFontIcon(
  'PhonelinkErase',
  'icon-phonelink_erase'
);
export const PhonelinkSetup = makeFontIcon(
  'PhonelinkSetup',
  'icon-phonelink_setup'
);
export const Planet = makeFontIcon('Planet', 'icon-planet');
export const Play = makeFontIcon('Play', 'icon-play');
export const PowerSwitch = makeFontIcon('PowerSwitch', 'icon-power-switch');
export const Profile = makeFontIcon('Profile', 'icon-profile');
export const Question = makeFontIcon('Question', 'icon-question-circle');
export const Refresh = makeFontIcon('Refresh', 'icon-redo2');
export const Restore = makeFontIcon('Restore', 'icon-restore');
export const Server = makeFontIcon('Server', 'icon-server');
export const SettingsInputComposite = makeFontIcon(
  'SettingsInputComposite',
  'icon-settings_input_composite'
);
export const SettingsOverscan = makeFontIcon(
  'SettingsOverscan',
  'icon-settings_overscan'
);
export const Share = makeFontIcon('Share', 'icon-share');
export const ShieldCheck = makeFontIcon('ShieldCheck', 'icon-shield-check');
export const Shrink = makeFontIcon('Shrink', 'icon-shrink');
export const SmallArrowDown = makeFontIcon(
  'SmallArrowDown',
  'icon-arrow_drop_down'
);
export const SmallArrowUp = makeFontIcon(
  'SmallArrowDown',
  'icon-arrow_drop_up'
);
export const Sort = makeFontIcon('Sort', 'icon-chevrons-expand-vertical');
export const SortAsc = makeFontIcon('SortAsc', 'icon-chevron-up');
export const SortDesc = makeFontIcon('SortDesc', 'icon-chevron-down');
export const Speed = makeFontIcon('Speed', 'icon-speed-fast');
export const Spinner = makeFontIcon('Spinner', 'icon-spinner8');
export const Stars = makeFontIcon('Stars', 'icon-stars');
export const Stripe = makeFontIcon('Stripe', 'icon-cc-stripe');
export const SyncAlt = makeFontIcon('SyncAlt', 'icon-sync2');
export const Tablet = makeFontIcon('Tablet', 'icon-tablet2');
export const Tags = makeFontIcon('Tags', 'icon-tags');
export const Terminal = makeFontIcon('Terminal', 'icon-cli');
export const Trash = makeFontIcon('Trash', 'icon-trash2');
export const Twitter = makeFontIcon('Twitter', 'icon-twitter');
export const UsbDrive = makeFontIcon('UsbDrive', 'icon-usb-drive');
export const Unarchive = makeFontIcon('Unarchive', 'icon-unarchive');
export const Unlock = makeFontIcon('Unlock', 'icon-unlock');
export const Unlink = makeFontIcon('Unlink', 'icon-unlink2');
export const Upload = makeFontIcon('Upload', 'icon-file_upload');
export const User = makeFontIcon('User', 'icon-user');
export const UserCreated = makeFontIcon('UserCreated', 'icon-user-created');
export const Users = makeFontIcon('Users', 'icon-users2');
export const VideoGame = makeFontIcon('VideoGame', 'icon-videogame_asset');
export const Visa = makeFontIcon('Visa', 'icon-cc-visa');
export const VolumeUp = makeFontIcon('VolumeUp', 'icon-volume-high');
export const VpnKey = makeFontIcon('VpnKey', 'icon-vpn_key');
export const Wand = makeFontIcon('Wand', 'icon-magic-wand');
export const Warning = makeFontIcon('Warning', 'icon-warning');
export const Wifi = makeFontIcon('Wifi', 'icon-wifi');
export const Windows = makeFontIcon('Windows', 'icon-windows');
export const Youtube = makeFontIcon('Youtube', 'icon-youtube');

export default Icon;
