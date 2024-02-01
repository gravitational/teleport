// package: teleport.accesslist.v1
// file: teleport/accesslist/v1/accesslist.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";
import * as google_protobuf_duration_pb from "google-protobuf/google/protobuf/duration_pb";
import * as google_protobuf_timestamp_pb from "google-protobuf/google/protobuf/timestamp_pb";
import * as teleport_header_v1_resourceheader_pb from "../../../teleport/header/v1/resourceheader_pb";
import * as teleport_trait_v1_trait_pb from "../../../teleport/trait/v1/trait_pb";

export class AccessList extends jspb.Message { 

    hasHeader(): boolean;
    clearHeader(): void;
    getHeader(): teleport_header_v1_resourceheader_pb.ResourceHeader | undefined;
    setHeader(value?: teleport_header_v1_resourceheader_pb.ResourceHeader): AccessList;


    hasSpec(): boolean;
    clearSpec(): void;
    getSpec(): AccessListSpec | undefined;
    setSpec(value?: AccessListSpec): AccessList;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): AccessList.AsObject;
    static toObject(includeInstance: boolean, msg: AccessList): AccessList.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: AccessList, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): AccessList;
    static deserializeBinaryFromReader(message: AccessList, reader: jspb.BinaryReader): AccessList;
}

export namespace AccessList {
    export type AsObject = {
        header?: teleport_header_v1_resourceheader_pb.ResourceHeader.AsObject,
        spec?: AccessListSpec.AsObject,
    }
}

export class AccessListSpec extends jspb.Message { 
    getDescription(): string;
    setDescription(value: string): AccessListSpec;

    clearOwnersList(): void;
    getOwnersList(): Array<AccessListOwner>;
    setOwnersList(value: Array<AccessListOwner>): AccessListSpec;
    addOwners(value?: AccessListOwner, index?: number): AccessListOwner;


    hasAudit(): boolean;
    clearAudit(): void;
    getAudit(): AccessListAudit | undefined;
    setAudit(value?: AccessListAudit): AccessListSpec;


    hasMembershipRequires(): boolean;
    clearMembershipRequires(): void;
    getMembershipRequires(): AccessListRequires | undefined;
    setMembershipRequires(value?: AccessListRequires): AccessListSpec;


    hasOwnershipRequires(): boolean;
    clearOwnershipRequires(): void;
    getOwnershipRequires(): AccessListRequires | undefined;
    setOwnershipRequires(value?: AccessListRequires): AccessListSpec;


    hasGrants(): boolean;
    clearGrants(): void;
    getGrants(): AccessListGrants | undefined;
    setGrants(value?: AccessListGrants): AccessListSpec;

    getTitle(): string;
    setTitle(value: string): AccessListSpec;

    getMembership(): string;
    setMembership(value: string): AccessListSpec;

    getOwnership(): string;
    setOwnership(value: string): AccessListSpec;


    hasOwnerGrants(): boolean;
    clearOwnerGrants(): void;
    getOwnerGrants(): AccessListGrants | undefined;
    setOwnerGrants(value?: AccessListGrants): AccessListSpec;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): AccessListSpec.AsObject;
    static toObject(includeInstance: boolean, msg: AccessListSpec): AccessListSpec.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: AccessListSpec, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): AccessListSpec;
    static deserializeBinaryFromReader(message: AccessListSpec, reader: jspb.BinaryReader): AccessListSpec;
}

export namespace AccessListSpec {
    export type AsObject = {
        description: string,
        ownersList: Array<AccessListOwner.AsObject>,
        audit?: AccessListAudit.AsObject,
        membershipRequires?: AccessListRequires.AsObject,
        ownershipRequires?: AccessListRequires.AsObject,
        grants?: AccessListGrants.AsObject,
        title: string,
        membership: string,
        ownership: string,
        ownerGrants?: AccessListGrants.AsObject,
    }
}

export class AccessListOwner extends jspb.Message { 
    getName(): string;
    setName(value: string): AccessListOwner;

    getDescription(): string;
    setDescription(value: string): AccessListOwner;

    getIneligibleStatus(): IneligibleStatus;
    setIneligibleStatus(value: IneligibleStatus): AccessListOwner;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): AccessListOwner.AsObject;
    static toObject(includeInstance: boolean, msg: AccessListOwner): AccessListOwner.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: AccessListOwner, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): AccessListOwner;
    static deserializeBinaryFromReader(message: AccessListOwner, reader: jspb.BinaryReader): AccessListOwner;
}

export namespace AccessListOwner {
    export type AsObject = {
        name: string,
        description: string,
        ineligibleStatus: IneligibleStatus,
    }
}

export class AccessListAudit extends jspb.Message { 

    hasNextAuditDate(): boolean;
    clearNextAuditDate(): void;
    getNextAuditDate(): google_protobuf_timestamp_pb.Timestamp | undefined;
    setNextAuditDate(value?: google_protobuf_timestamp_pb.Timestamp): AccessListAudit;


    hasRecurrence(): boolean;
    clearRecurrence(): void;
    getRecurrence(): Recurrence | undefined;
    setRecurrence(value?: Recurrence): AccessListAudit;


    hasNotifications(): boolean;
    clearNotifications(): void;
    getNotifications(): Notifications | undefined;
    setNotifications(value?: Notifications): AccessListAudit;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): AccessListAudit.AsObject;
    static toObject(includeInstance: boolean, msg: AccessListAudit): AccessListAudit.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: AccessListAudit, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): AccessListAudit;
    static deserializeBinaryFromReader(message: AccessListAudit, reader: jspb.BinaryReader): AccessListAudit;
}

export namespace AccessListAudit {
    export type AsObject = {
        nextAuditDate?: google_protobuf_timestamp_pb.Timestamp.AsObject,
        recurrence?: Recurrence.AsObject,
        notifications?: Notifications.AsObject,
    }
}

export class Recurrence extends jspb.Message { 
    getFrequency(): ReviewFrequency;
    setFrequency(value: ReviewFrequency): Recurrence;

    getDayOfMonth(): ReviewDayOfMonth;
    setDayOfMonth(value: ReviewDayOfMonth): Recurrence;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Recurrence.AsObject;
    static toObject(includeInstance: boolean, msg: Recurrence): Recurrence.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Recurrence, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Recurrence;
    static deserializeBinaryFromReader(message: Recurrence, reader: jspb.BinaryReader): Recurrence;
}

export namespace Recurrence {
    export type AsObject = {
        frequency: ReviewFrequency,
        dayOfMonth: ReviewDayOfMonth,
    }
}

export class Notifications extends jspb.Message { 

    hasStart(): boolean;
    clearStart(): void;
    getStart(): google_protobuf_duration_pb.Duration | undefined;
    setStart(value?: google_protobuf_duration_pb.Duration): Notifications;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Notifications.AsObject;
    static toObject(includeInstance: boolean, msg: Notifications): Notifications.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Notifications, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Notifications;
    static deserializeBinaryFromReader(message: Notifications, reader: jspb.BinaryReader): Notifications;
}

export namespace Notifications {
    export type AsObject = {
        start?: google_protobuf_duration_pb.Duration.AsObject,
    }
}

export class AccessListRequires extends jspb.Message { 
    clearRolesList(): void;
    getRolesList(): Array<string>;
    setRolesList(value: Array<string>): AccessListRequires;
    addRoles(value: string, index?: number): string;

    clearTraitsList(): void;
    getTraitsList(): Array<teleport_trait_v1_trait_pb.Trait>;
    setTraitsList(value: Array<teleport_trait_v1_trait_pb.Trait>): AccessListRequires;
    addTraits(value?: teleport_trait_v1_trait_pb.Trait, index?: number): teleport_trait_v1_trait_pb.Trait;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): AccessListRequires.AsObject;
    static toObject(includeInstance: boolean, msg: AccessListRequires): AccessListRequires.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: AccessListRequires, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): AccessListRequires;
    static deserializeBinaryFromReader(message: AccessListRequires, reader: jspb.BinaryReader): AccessListRequires;
}

export namespace AccessListRequires {
    export type AsObject = {
        rolesList: Array<string>,
        traitsList: Array<teleport_trait_v1_trait_pb.Trait.AsObject>,
    }
}

export class AccessListGrants extends jspb.Message { 
    clearRolesList(): void;
    getRolesList(): Array<string>;
    setRolesList(value: Array<string>): AccessListGrants;
    addRoles(value: string, index?: number): string;

    clearTraitsList(): void;
    getTraitsList(): Array<teleport_trait_v1_trait_pb.Trait>;
    setTraitsList(value: Array<teleport_trait_v1_trait_pb.Trait>): AccessListGrants;
    addTraits(value?: teleport_trait_v1_trait_pb.Trait, index?: number): teleport_trait_v1_trait_pb.Trait;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): AccessListGrants.AsObject;
    static toObject(includeInstance: boolean, msg: AccessListGrants): AccessListGrants.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: AccessListGrants, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): AccessListGrants;
    static deserializeBinaryFromReader(message: AccessListGrants, reader: jspb.BinaryReader): AccessListGrants;
}

export namespace AccessListGrants {
    export type AsObject = {
        rolesList: Array<string>,
        traitsList: Array<teleport_trait_v1_trait_pb.Trait.AsObject>,
    }
}

export class Member extends jspb.Message { 

    hasHeader(): boolean;
    clearHeader(): void;
    getHeader(): teleport_header_v1_resourceheader_pb.ResourceHeader | undefined;
    setHeader(value?: teleport_header_v1_resourceheader_pb.ResourceHeader): Member;


    hasSpec(): boolean;
    clearSpec(): void;
    getSpec(): MemberSpec | undefined;
    setSpec(value?: MemberSpec): Member;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Member.AsObject;
    static toObject(includeInstance: boolean, msg: Member): Member.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Member, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Member;
    static deserializeBinaryFromReader(message: Member, reader: jspb.BinaryReader): Member;
}

export namespace Member {
    export type AsObject = {
        header?: teleport_header_v1_resourceheader_pb.ResourceHeader.AsObject,
        spec?: MemberSpec.AsObject,
    }
}

export class MemberSpec extends jspb.Message { 
    getAccessList(): string;
    setAccessList(value: string): MemberSpec;

    getName(): string;
    setName(value: string): MemberSpec;


    hasJoined(): boolean;
    clearJoined(): void;
    getJoined(): google_protobuf_timestamp_pb.Timestamp | undefined;
    setJoined(value?: google_protobuf_timestamp_pb.Timestamp): MemberSpec;


    hasExpires(): boolean;
    clearExpires(): void;
    getExpires(): google_protobuf_timestamp_pb.Timestamp | undefined;
    setExpires(value?: google_protobuf_timestamp_pb.Timestamp): MemberSpec;

    getReason(): string;
    setReason(value: string): MemberSpec;

    getAddedBy(): string;
    setAddedBy(value: string): MemberSpec;

    getIneligibleStatus(): IneligibleStatus;
    setIneligibleStatus(value: IneligibleStatus): MemberSpec;

    getMembership(): string;
    setMembership(value: string): MemberSpec;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): MemberSpec.AsObject;
    static toObject(includeInstance: boolean, msg: MemberSpec): MemberSpec.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: MemberSpec, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): MemberSpec;
    static deserializeBinaryFromReader(message: MemberSpec, reader: jspb.BinaryReader): MemberSpec;
}

export namespace MemberSpec {
    export type AsObject = {
        accessList: string,
        name: string,
        joined?: google_protobuf_timestamp_pb.Timestamp.AsObject,
        expires?: google_protobuf_timestamp_pb.Timestamp.AsObject,
        reason: string,
        addedBy: string,
        ineligibleStatus: IneligibleStatus,
        membership: string,
    }
}

export class Review extends jspb.Message { 

    hasHeader(): boolean;
    clearHeader(): void;
    getHeader(): teleport_header_v1_resourceheader_pb.ResourceHeader | undefined;
    setHeader(value?: teleport_header_v1_resourceheader_pb.ResourceHeader): Review;


    hasSpec(): boolean;
    clearSpec(): void;
    getSpec(): ReviewSpec | undefined;
    setSpec(value?: ReviewSpec): Review;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Review.AsObject;
    static toObject(includeInstance: boolean, msg: Review): Review.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Review, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Review;
    static deserializeBinaryFromReader(message: Review, reader: jspb.BinaryReader): Review;
}

export namespace Review {
    export type AsObject = {
        header?: teleport_header_v1_resourceheader_pb.ResourceHeader.AsObject,
        spec?: ReviewSpec.AsObject,
    }
}

export class ReviewSpec extends jspb.Message { 
    getAccessList(): string;
    setAccessList(value: string): ReviewSpec;

    clearReviewersList(): void;
    getReviewersList(): Array<string>;
    setReviewersList(value: Array<string>): ReviewSpec;
    addReviewers(value: string, index?: number): string;


    hasReviewDate(): boolean;
    clearReviewDate(): void;
    getReviewDate(): google_protobuf_timestamp_pb.Timestamp | undefined;
    setReviewDate(value?: google_protobuf_timestamp_pb.Timestamp): ReviewSpec;

    getNotes(): string;
    setNotes(value: string): ReviewSpec;


    hasChanges(): boolean;
    clearChanges(): void;
    getChanges(): ReviewChanges | undefined;
    setChanges(value?: ReviewChanges): ReviewSpec;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ReviewSpec.AsObject;
    static toObject(includeInstance: boolean, msg: ReviewSpec): ReviewSpec.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ReviewSpec, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ReviewSpec;
    static deserializeBinaryFromReader(message: ReviewSpec, reader: jspb.BinaryReader): ReviewSpec;
}

export namespace ReviewSpec {
    export type AsObject = {
        accessList: string,
        reviewersList: Array<string>,
        reviewDate?: google_protobuf_timestamp_pb.Timestamp.AsObject,
        notes: string,
        changes?: ReviewChanges.AsObject,
    }
}

export class ReviewChanges extends jspb.Message { 

    hasMembershipRequirementsChanged(): boolean;
    clearMembershipRequirementsChanged(): void;
    getMembershipRequirementsChanged(): AccessListRequires | undefined;
    setMembershipRequirementsChanged(value?: AccessListRequires): ReviewChanges;

    clearRemovedMembersList(): void;
    getRemovedMembersList(): Array<string>;
    setRemovedMembersList(value: Array<string>): ReviewChanges;
    addRemovedMembers(value: string, index?: number): string;

    getReviewFrequencyChanged(): ReviewFrequency;
    setReviewFrequencyChanged(value: ReviewFrequency): ReviewChanges;

    getReviewDayOfMonthChanged(): ReviewDayOfMonth;
    setReviewDayOfMonthChanged(value: ReviewDayOfMonth): ReviewChanges;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ReviewChanges.AsObject;
    static toObject(includeInstance: boolean, msg: ReviewChanges): ReviewChanges.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ReviewChanges, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ReviewChanges;
    static deserializeBinaryFromReader(message: ReviewChanges, reader: jspb.BinaryReader): ReviewChanges;
}

export namespace ReviewChanges {
    export type AsObject = {
        membershipRequirementsChanged?: AccessListRequires.AsObject,
        removedMembersList: Array<string>,
        reviewFrequencyChanged: ReviewFrequency,
        reviewDayOfMonthChanged: ReviewDayOfMonth,
    }
}

export enum ReviewFrequency {
    REVIEW_FREQUENCY_UNSPECIFIED = 0,
    REVIEW_FREQUENCY_ONE_MONTH = 1,
    REVIEW_FREQUENCY_THREE_MONTHS = 3,
    REVIEW_FREQUENCY_SIX_MONTHS = 6,
    REVIEW_FREQUENCY_ONE_YEAR = 12,
}

export enum ReviewDayOfMonth {
    REVIEW_DAY_OF_MONTH_UNSPECIFIED = 0,
    REVIEW_DAY_OF_MONTH_FIRST = 1,
    REVIEW_DAY_OF_MONTH_FIFTEENTH = 15,
    REVIEW_DAY_OF_MONTH_LAST = 31,
}

export enum IneligibleStatus {
    INELIGIBLE_STATUS_UNSPECIFIED = 0,
    INELIGIBLE_STATUS_ELIGIBLE = 1,
    INELIGIBLE_STATUS_USER_NOT_EXIST = 2,
    INELIGIBLE_STATUS_MISSING_REQUIREMENTS = 3,
    INELIGIBLE_STATUS_EXPIRED = 4,
}
