// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

import Foundation
import os

private let logger = Logger(
    subsystem: Bundle.main.bundleIdentifier ?? "com.gravitational.verify",
    category: "deep_link"
)

func parseDeepLink(_ url: URL) -> DeepLinkParseResult {
    let path = url.path(percentEncoded: false)
    switch path {
    case "/enroll_mobile_device":
        // Make sure to first switch on the path and only then attempt to parse out individual fields
        // out of the URL. This way this function always returns the most important error first,
        // which is the error about the unsupported path.
        return parseDeepURL(url) { deepURL, parts in
            guard
                let enrollPairingToken = getQueryParam(
                    parts,
                    "enroll_pairing_token"
                )
            else {
                return .failure(.missingPart("enroll pairing token"))
            }
            return .success(
                .enrollMobileDevice(
                    EnrollMobileDeviceDeepURL(
                        url: deepURL,
                        enrollPairingToken: enrollPairingToken
                    )
                )
            )
        }
    default:
        logger.warning("Unsupported path: \(path, privacy: .public)")
        return .failure(.unsupportedPath)
    }
}

typealias DeepLinkParseResult = Result<ParsedDeepLink, DeepLinkParseError>

enum ParsedDeepLink {
    case enrollMobileDevice(EnrollMobileDeviceDeepURL)
}

enum DeepLinkParseError: Error {
    case unsupportedPath
    case urlComponentsFailed
    case missingPart(String)
}

struct DeepURL {
    /// hostname is just the name of the host without the port.
    var hostname: String
    /// port of the host.
    var port: Int?
}

struct EnrollMobileDeviceDeepURL {
    var url: DeepURL
    /// user_token query params from the deep link.
    var enrollPairingToken: String
}

/// parseDeepURL could return Result<(DeepURL, URLComponents), DeepLinkParseError> instead, but
/// there's no convienient way to unwrap it, that's why it accepts a trailing closure instead.
func parseDeepURL(
    _ url: URL,
    toParsedDeepLink: (_ deepURL: DeepURL, _ parts: URLComponents) ->
        DeepLinkParseResult
) -> DeepLinkParseResult {
    guard let parts = URLComponents(string: url.absoluteString) else {
        return .failure(.urlComponentsFailed)
    }
    guard let hostname = parts.host, hostname != "" else {
        return .failure(.missingPart("hostname"))
    }
    let port = parts.port

    return toParsedDeepLink(DeepURL(hostname: hostname, port: port), parts)
}

func getQueryParam(_ parts: URLComponents, _ param: String) -> String? {
    parts.queryItems?
        .first(where: { $0.name == param && $0.value != nil && $0.value != "" }
        )?.value
}
