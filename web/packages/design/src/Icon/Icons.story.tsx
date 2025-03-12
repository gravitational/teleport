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

import { ComponentType } from 'react';

import * as Icon from '.';
import { Text } from '..';
import Flex from './../Flex';
import { IconProps } from './Icon';

export default {
  title: 'Design/Icon',
};

/*

THIS FILE IS GENERATED. DO NOT EDIT.

*/

export const Icons = () => (
  <Flex flexWrap="wrap">
    <IconBox IconCmpt={Icon.Add} text="Add" />
    <IconBox IconCmpt={Icon.AddCircle} text="AddCircle" />
    <IconBox IconCmpt={Icon.AddUsers} text="AddUsers" />
    <IconBox IconCmpt={Icon.AlarmRing} text="AlarmRing" />
    <IconBox IconCmpt={Icon.AmazonAws} text="AmazonAws" />
    <IconBox IconCmpt={Icon.Apartment} text="Apartment" />
    <IconBox IconCmpt={Icon.Apple} text="Apple" />
    <IconBox IconCmpt={Icon.Application} text="Application" />
    <IconBox IconCmpt={Icon.Archive} text="Archive" />
    <IconBox IconCmpt={Icon.ArrowBack} text="ArrowBack" />
    <IconBox IconCmpt={Icon.ArrowDown} text="ArrowDown" />
    <IconBox IconCmpt={Icon.ArrowFatLinesUp} text="ArrowFatLinesUp" />
    <IconBox IconCmpt={Icon.ArrowForward} text="ArrowForward" />
    <IconBox IconCmpt={Icon.ArrowLeft} text="ArrowLeft" />
    <IconBox IconCmpt={Icon.ArrowLineLeft} text="ArrowLineLeft" />
    <IconBox IconCmpt={Icon.ArrowRight} text="ArrowRight" />
    <IconBox IconCmpt={Icon.ArrowSquareIn} text="ArrowSquareIn" />
    <IconBox IconCmpt={Icon.ArrowSquareOut} text="ArrowSquareOut" />
    <IconBox IconCmpt={Icon.ArrowUp} text="ArrowUp" />
    <IconBox IconCmpt={Icon.ArrowsIn} text="ArrowsIn" />
    <IconBox IconCmpt={Icon.ArrowsOut} text="ArrowsOut" />
    <IconBox IconCmpt={Icon.BellRinging} text="BellRinging" />
    <IconBox IconCmpt={Icon.BookOpenText} text="BookOpenText" />
    <IconBox IconCmpt={Icon.Bots} text="Bots" />
    <IconBox IconCmpt={Icon.Broadcast} text="Broadcast" />
    <IconBox IconCmpt={Icon.BroadcastSlash} text="BroadcastSlash" />
    <IconBox IconCmpt={Icon.Bubble} text="Bubble" />
    <IconBox IconCmpt={Icon.CCAmex} text="CCAmex" />
    <IconBox IconCmpt={Icon.CCDiscover} text="CCDiscover" />
    <IconBox IconCmpt={Icon.CCMasterCard} text="CCMasterCard" />
    <IconBox IconCmpt={Icon.CCStripe} text="CCStripe" />
    <IconBox IconCmpt={Icon.CCVisa} text="CCVisa" />
    <IconBox IconCmpt={Icon.Calendar} text="Calendar" />
    <IconBox IconCmpt={Icon.Camera} text="Camera" />
    <IconBox IconCmpt={Icon.CardView} text="CardView" />
    <IconBox IconCmpt={Icon.Cash} text="Cash" />
    <IconBox IconCmpt={Icon.Chart} text="Chart" />
    <IconBox IconCmpt={Icon.ChatBubble} text="ChatBubble" />
    <IconBox IconCmpt={Icon.ChatCircleSparkle} text="ChatCircleSparkle" />
    <IconBox IconCmpt={Icon.Check} text="Check" />
    <IconBox IconCmpt={Icon.CheckThick} text="CheckThick" />
    <IconBox IconCmpt={Icon.Checks} text="Checks" />
    <IconBox IconCmpt={Icon.ChevronCircleDown} text="ChevronCircleDown" />
    <IconBox IconCmpt={Icon.ChevronCircleLeft} text="ChevronCircleLeft" />
    <IconBox IconCmpt={Icon.ChevronCircleRight} text="ChevronCircleRight" />
    <IconBox IconCmpt={Icon.ChevronCircleUp} text="ChevronCircleUp" />
    <IconBox IconCmpt={Icon.ChevronDown} text="ChevronDown" />
    <IconBox IconCmpt={Icon.ChevronLeft} text="ChevronLeft" />
    <IconBox IconCmpt={Icon.ChevronRight} text="ChevronRight" />
    <IconBox IconCmpt={Icon.ChevronUp} text="ChevronUp" />
    <IconBox IconCmpt={Icon.ChevronsVertical} text="ChevronsVertical" />
    <IconBox IconCmpt={Icon.CircleArrowLeft} text="CircleArrowLeft" />
    <IconBox IconCmpt={Icon.CircleArrowRight} text="CircleArrowRight" />
    <IconBox IconCmpt={Icon.CircleCheck} text="CircleCheck" />
    <IconBox IconCmpt={Icon.CircleCross} text="CircleCross" />
    <IconBox IconCmpt={Icon.CirclePause} text="CirclePause" />
    <IconBox IconCmpt={Icon.CirclePlay} text="CirclePlay" />
    <IconBox IconCmpt={Icon.CircleStop} text="CircleStop" />
    <IconBox IconCmpt={Icon.Cli} text="Cli" />
    <IconBox IconCmpt={Icon.Clipboard} text="Clipboard" />
    <IconBox IconCmpt={Icon.ClipboardUser} text="ClipboardUser" />
    <IconBox IconCmpt={Icon.Clock} text="Clock" />
    <IconBox IconCmpt={Icon.Cloud} text="Cloud" />
    <IconBox IconCmpt={Icon.Cluster} text="Cluster" />
    <IconBox IconCmpt={Icon.Code} text="Code" />
    <IconBox IconCmpt={Icon.Cog} text="Cog" />
    <IconBox IconCmpt={Icon.Config} text="Config" />
    <IconBox IconCmpt={Icon.Contract} text="Contract" />
    <IconBox IconCmpt={Icon.Copy} text="Copy" />
    <IconBox IconCmpt={Icon.CreditCard} text="CreditCard" />
    <IconBox IconCmpt={Icon.Cross} text="Cross" />
    <IconBox IconCmpt={Icon.Crown} text="Crown" />
    <IconBox IconCmpt={Icon.Database} text="Database" />
    <IconBox IconCmpt={Icon.Desktop} text="Desktop" />
    <IconBox IconCmpt={Icon.DeviceMobileCamera} text="DeviceMobileCamera" />
    <IconBox IconCmpt={Icon.Devices} text="Devices" />
    <IconBox IconCmpt={Icon.Download} text="Download" />
    <IconBox IconCmpt={Icon.Earth} text="Earth" />
    <IconBox IconCmpt={Icon.Edit} text="Edit" />
    <IconBox IconCmpt={Icon.Ellipsis} text="Ellipsis" />
    <IconBox IconCmpt={Icon.EmailSolid} text="EmailSolid" />
    <IconBox IconCmpt={Icon.EnvelopeOpen} text="EnvelopeOpen" />
    <IconBox IconCmpt={Icon.EqualizersVertical} text="EqualizersVertical" />
    <IconBox IconCmpt={Icon.Expand} text="Expand" />
    <IconBox IconCmpt={Icon.Facebook} text="Facebook" />
    <IconBox IconCmpt={Icon.FingerprintSimple} text="FingerprintSimple" />
    <IconBox IconCmpt={Icon.Floppy} text="Floppy" />
    <IconBox IconCmpt={Icon.FlowArrow} text="FlowArrow" />
    <IconBox IconCmpt={Icon.FolderPlus} text="FolderPlus" />
    <IconBox IconCmpt={Icon.FolderShared} text="FolderShared" />
    <IconBox IconCmpt={Icon.GitHub} text="GitHub" />
    <IconBox IconCmpt={Icon.Google} text="Google" />
    <IconBox IconCmpt={Icon.Graph} text="Graph" />
    <IconBox IconCmpt={Icon.Hashtag} text="Hashtag" />
    <IconBox IconCmpt={Icon.Headset} text="Headset" />
    <IconBox IconCmpt={Icon.Home} text="Home" />
    <IconBox IconCmpt={Icon.Info} text="Info" />
    <IconBox IconCmpt={Icon.Integrations} text="Integrations" />
    <IconBox IconCmpt={Icon.Invoices} text="Invoices" />
    <IconBox IconCmpt={Icon.Key} text="Key" />
    <IconBox IconCmpt={Icon.KeyHole} text="KeyHole" />
    <IconBox IconCmpt={Icon.Keyboard} text="Keyboard" />
    <IconBox IconCmpt={Icon.Keypair} text="Keypair" />
    <IconBox IconCmpt={Icon.Kubernetes} text="Kubernetes" />
    <IconBox IconCmpt={Icon.Label} text="Label" />
    <IconBox IconCmpt={Icon.Lan} text="Lan" />
    <IconBox IconCmpt={Icon.Laptop} text="Laptop" />
    <IconBox IconCmpt={Icon.Layout} text="Layout" />
    <IconBox IconCmpt={Icon.License} text="License" />
    <IconBox IconCmpt={Icon.LineSegment} text="LineSegment" />
    <IconBox IconCmpt={Icon.LineSegments} text="LineSegments" />
    <IconBox IconCmpt={Icon.Link} text="Link" />
    <IconBox IconCmpt={Icon.Linkedin} text="Linkedin" />
    <IconBox IconCmpt={Icon.Linux} text="Linux" />
    <IconBox IconCmpt={Icon.ListAddCheck} text="ListAddCheck" />
    <IconBox IconCmpt={Icon.ListMagnifyingGlass} text="ListMagnifyingGlass" />
    <IconBox IconCmpt={Icon.ListThin} text="ListThin" />
    <IconBox IconCmpt={Icon.ListView} text="ListView" />
    <IconBox IconCmpt={Icon.Lock} text="Lock" />
    <IconBox IconCmpt={Icon.LockKey} text="LockKey" />
    <IconBox IconCmpt={Icon.Logout} text="Logout" />
    <IconBox IconCmpt={Icon.Magnifier} text="Magnifier" />
    <IconBox IconCmpt={Icon.MagnifyingMinus} text="MagnifyingMinus" />
    <IconBox IconCmpt={Icon.MagnifyingPlus} text="MagnifyingPlus" />
    <IconBox IconCmpt={Icon.Memory} text="Memory" />
    <IconBox IconCmpt={Icon.Minus} text="Minus" />
    <IconBox IconCmpt={Icon.MinusCircle} text="MinusCircle" />
    <IconBox IconCmpt={Icon.Moon} text="Moon" />
    <IconBox IconCmpt={Icon.MoreHoriz} text="MoreHoriz" />
    <IconBox IconCmpt={Icon.MoreVert} text="MoreVert" />
    <IconBox IconCmpt={Icon.Mute} text="Mute" />
    <IconBox IconCmpt={Icon.NewTab} text="NewTab" />
    <IconBox IconCmpt={Icon.NoteAdded} text="NoteAdded" />
    <IconBox IconCmpt={Icon.Notification} text="Notification" />
    <IconBox IconCmpt={Icon.NotificationsActive} text="NotificationsActive" />
    <IconBox IconCmpt={Icon.PaperPlane} text="PaperPlane" />
    <IconBox IconCmpt={Icon.Password} text="Password" />
    <IconBox IconCmpt={Icon.Pencil} text="Pencil" />
    <IconBox IconCmpt={Icon.Planet} text="Planet" />
    <IconBox IconCmpt={Icon.Plugs} text="Plugs" />
    <IconBox IconCmpt={Icon.PlugsConnected} text="PlugsConnected" />
    <IconBox IconCmpt={Icon.Plus} text="Plus" />
    <IconBox IconCmpt={Icon.PowerSwitch} text="PowerSwitch" />
    <IconBox IconCmpt={Icon.Printer} text="Printer" />
    <IconBox IconCmpt={Icon.Profile} text="Profile" />
    <IconBox IconCmpt={Icon.PushPin} text="PushPin" />
    <IconBox IconCmpt={Icon.PushPinFilled} text="PushPinFilled" />
    <IconBox IconCmpt={Icon.Question} text="Question" />
    <IconBox IconCmpt={Icon.Refresh} text="Refresh" />
    <IconBox IconCmpt={Icon.Restore} text="Restore" />
    <IconBox IconCmpt={Icon.RocketLaunch} text="RocketLaunch" />
    <IconBox IconCmpt={Icon.Rows} text="Rows" />
    <IconBox IconCmpt={Icon.Ruler} text="Ruler" />
    <IconBox IconCmpt={Icon.Run} text="Run" />
    <IconBox IconCmpt={Icon.Scan} text="Scan" />
    <IconBox IconCmpt={Icon.Server} text="Server" />
    <IconBox IconCmpt={Icon.Share} text="Share" />
    <IconBox IconCmpt={Icon.ShieldCheck} text="ShieldCheck" />
    <IconBox IconCmpt={Icon.ShieldWarning} text="ShieldWarning" />
    <IconBox IconCmpt={Icon.Sliders} text="Sliders" />
    <IconBox IconCmpt={Icon.SlidersVertical} text="SlidersVertical" />
    <IconBox IconCmpt={Icon.Speed} text="Speed" />
    <IconBox IconCmpt={Icon.Spinner} text="Spinner" />
    <IconBox IconCmpt={Icon.SquaresFour} text="SquaresFour" />
    <IconBox IconCmpt={Icon.Stars} text="Stars" />
    <IconBox IconCmpt={Icon.Sun} text="Sun" />
    <IconBox IconCmpt={Icon.SyncAlt} text="SyncAlt" />
    <IconBox IconCmpt={Icon.Table} text="Table" />
    <IconBox IconCmpt={Icon.Tablet} text="Tablet" />
    <IconBox IconCmpt={Icon.Tags} text="Tags" />
    <IconBox IconCmpt={Icon.Terminal} text="Terminal" />
    <IconBox IconCmpt={Icon.Trash} text="Trash" />
    <IconBox IconCmpt={Icon.Twitter} text="Twitter" />
    <IconBox IconCmpt={Icon.Unarchive} text="Unarchive" />
    <IconBox IconCmpt={Icon.Unlink} text="Unlink" />
    <IconBox IconCmpt={Icon.Unlock} text="Unlock" />
    <IconBox IconCmpt={Icon.Upload} text="Upload" />
    <IconBox IconCmpt={Icon.UsbDrive} text="UsbDrive" />
    <IconBox IconCmpt={Icon.User} text="User" />
    <IconBox IconCmpt={Icon.UserAdd} text="UserAdd" />
    <IconBox IconCmpt={Icon.UserCircleGear} text="UserCircleGear" />
    <IconBox IconCmpt={Icon.UserFocus} text="UserFocus" />
    <IconBox IconCmpt={Icon.UserIdBadge} text="UserIdBadge" />
    <IconBox IconCmpt={Icon.UserList} text="UserList" />
    <IconBox IconCmpt={Icon.Users} text="Users" />
    <IconBox IconCmpt={Icon.UsersTriple} text="UsersTriple" />
    <IconBox IconCmpt={Icon.Vault} text="Vault" />
    <IconBox IconCmpt={Icon.VideoGame} text="VideoGame" />
    <IconBox IconCmpt={Icon.VolumeUp} text="VolumeUp" />
    <IconBox IconCmpt={Icon.VpnKey} text="VpnKey" />
    <IconBox IconCmpt={Icon.Wand} text="Wand" />
    <IconBox IconCmpt={Icon.Warning} text="Warning" />
    <IconBox IconCmpt={Icon.WarningCircle} text="WarningCircle" />
    <IconBox IconCmpt={Icon.Wifi} text="Wifi" />
    <IconBox IconCmpt={Icon.Windows} text="Windows" />
    <IconBox IconCmpt={Icon.Wrench} text="Wrench" />
    <IconBox IconCmpt={Icon.Youtube} text="Youtube" />
  </Flex>
);

const IconBox = ({
  IconCmpt,
  text,
}: {
  IconCmpt: ComponentType<IconProps>;
  text: string;
}) => (
  <Flex m="10px" width="300px">
    <IconCmpt />
    <Text ml={2}>{text}</Text>
  </Flex>
);
