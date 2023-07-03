// package: prehog.v1
// file: prehog/v1/teleport.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";
import * as google_protobuf_timestamp_pb from "google-protobuf/google/protobuf/timestamp_pb";

export class UserActivityReport extends jspb.Message { 
    getReportUuid(): Uint8Array | string;
    getReportUuid_asU8(): Uint8Array;
    getReportUuid_asB64(): string;
    setReportUuid(value: Uint8Array | string): UserActivityReport;

    getClusterName(): Uint8Array | string;
    getClusterName_asU8(): Uint8Array;
    getClusterName_asB64(): string;
    setClusterName(value: Uint8Array | string): UserActivityReport;

    getReporterHostid(): Uint8Array | string;
    getReporterHostid_asU8(): Uint8Array;
    getReporterHostid_asB64(): string;
    setReporterHostid(value: Uint8Array | string): UserActivityReport;


    hasStartTime(): boolean;
    clearStartTime(): void;
    getStartTime(): google_protobuf_timestamp_pb.Timestamp | undefined;
    setStartTime(value?: google_protobuf_timestamp_pb.Timestamp): UserActivityReport;

    clearRecordsList(): void;
    getRecordsList(): Array<UserActivityRecord>;
    setRecordsList(value: Array<UserActivityRecord>): UserActivityReport;
    addRecords(value?: UserActivityRecord, index?: number): UserActivityRecord;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): UserActivityReport.AsObject;
    static toObject(includeInstance: boolean, msg: UserActivityReport): UserActivityReport.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: UserActivityReport, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): UserActivityReport;
    static deserializeBinaryFromReader(message: UserActivityReport, reader: jspb.BinaryReader): UserActivityReport;
}

export namespace UserActivityReport {
    export type AsObject = {
        reportUuid: Uint8Array | string,
        clusterName: Uint8Array | string,
        reporterHostid: Uint8Array | string,
        startTime?: google_protobuf_timestamp_pb.Timestamp.AsObject,
        recordsList: Array<UserActivityRecord.AsObject>,
    }
}

export class UserActivityRecord extends jspb.Message { 
    getUserName(): Uint8Array | string;
    getUserName_asU8(): Uint8Array;
    getUserName_asB64(): string;
    setUserName(value: Uint8Array | string): UserActivityRecord;

    getLogins(): number;
    setLogins(value: number): UserActivityRecord;

    getSshSessions(): number;
    setSshSessions(value: number): UserActivityRecord;

    getAppSessions(): number;
    setAppSessions(value: number): UserActivityRecord;

    getKubeSessions(): number;
    setKubeSessions(value: number): UserActivityRecord;

    getDbSessions(): number;
    setDbSessions(value: number): UserActivityRecord;

    getDesktopSessions(): number;
    setDesktopSessions(value: number): UserActivityRecord;

    getAppTcpSessions(): number;
    setAppTcpSessions(value: number): UserActivityRecord;

    getSshPortSessions(): number;
    setSshPortSessions(value: number): UserActivityRecord;

    getKubeRequests(): number;
    setKubeRequests(value: number): UserActivityRecord;

    getSftpEvents(): number;
    setSftpEvents(value: number): UserActivityRecord;

    getSshPortV2Sessions(): number;
    setSshPortV2Sessions(value: number): UserActivityRecord;

    getKubePortSessions(): number;
    setKubePortSessions(value: number): UserActivityRecord;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): UserActivityRecord.AsObject;
    static toObject(includeInstance: boolean, msg: UserActivityRecord): UserActivityRecord.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: UserActivityRecord, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): UserActivityRecord;
    static deserializeBinaryFromReader(message: UserActivityRecord, reader: jspb.BinaryReader): UserActivityRecord;
}

export namespace UserActivityRecord {
    export type AsObject = {
        userName: Uint8Array | string,
        logins: number,
        sshSessions: number,
        appSessions: number,
        kubeSessions: number,
        dbSessions: number,
        desktopSessions: number,
        appTcpSessions: number,
        sshPortSessions: number,
        kubeRequests: number,
        sftpEvents: number,
        sshPortV2Sessions: number,
        kubePortSessions: number,
    }
}

export class SubmitUsageReportsRequest extends jspb.Message { 
    clearUserActivityList(): void;
    getUserActivityList(): Array<UserActivityReport>;
    setUserActivityList(value: Array<UserActivityReport>): SubmitUsageReportsRequest;
    addUserActivity(value?: UserActivityReport, index?: number): UserActivityReport;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): SubmitUsageReportsRequest.AsObject;
    static toObject(includeInstance: boolean, msg: SubmitUsageReportsRequest): SubmitUsageReportsRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: SubmitUsageReportsRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): SubmitUsageReportsRequest;
    static deserializeBinaryFromReader(message: SubmitUsageReportsRequest, reader: jspb.BinaryReader): SubmitUsageReportsRequest;
}

export namespace SubmitUsageReportsRequest {
    export type AsObject = {
        userActivityList: Array<UserActivityReport.AsObject>,
    }
}

export class SubmitUsageReportsResponse extends jspb.Message { 
    getBatchUuid(): Uint8Array | string;
    getBatchUuid_asU8(): Uint8Array;
    getBatchUuid_asB64(): string;
    setBatchUuid(value: Uint8Array | string): SubmitUsageReportsResponse;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): SubmitUsageReportsResponse.AsObject;
    static toObject(includeInstance: boolean, msg: SubmitUsageReportsResponse): SubmitUsageReportsResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: SubmitUsageReportsResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): SubmitUsageReportsResponse;
    static deserializeBinaryFromReader(message: SubmitUsageReportsResponse, reader: jspb.BinaryReader): SubmitUsageReportsResponse;
}

export namespace SubmitUsageReportsResponse {
    export type AsObject = {
        batchUuid: Uint8Array | string,
    }
}
