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

enum DeepLink: Equatable {
	case enrollMobileDevice(EnrollMobileDeviceDeepLink)

	init(from url: URL) throws {
		switch url.path(percentEncoded: false) {
			case "/enroll_mobile_device":
				// Make sure to first switch on the path and only then attempt to parse out individual fields
				// out of the URL. This way this function always returns the most important error first,
				// which is the error about the unsupported path.
				self = try .enrollMobileDevice(EnrollMobileDeviceDeepLink(from: url))
			default:
				throw DeepLinkParseError.unsupportedPath
		}
	}
}

enum DeepLinkParseError: LocalizedError, Equatable {
	case unsupportedPath
	case urlComponentsFailed
	case missingPart(String)

	var errorDescription: String? {
		switch self {
			case .unsupportedPath:
				NSLocalizedString(
					"This version of the app does not support the action represented by this link.",
					comment: "An error message that appears when a user tries to open a deep link with an unsupported path.",
				)
			case .urlComponentsFailed:
				NSLocalizedString(
					"The link appears to be malformed and could not be parsed.",
					comment: "An error message that appears when a user tries to open a malformed deep link.",
				)
			case let .missingPart(part):
				String(
					format: NSLocalizedString(
						"The %@ part of the link is missing.",
						comment: "An error message that appears when a user tries to open a deep link with a missing required part.",
					),
					part,
				)
		}
	}
}
