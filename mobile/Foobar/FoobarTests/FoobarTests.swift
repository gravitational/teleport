// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

//
//  FoobarTests.swift
//  FoobarTests
//
//  Created by Rafał Cieślak on 2025-09-11.
//

@testable import Foobar
import Foundation
import Testing

struct FoobarTests {
  @Test func attemptEquatable() async throws {
    // Write your test here and use APIs like `#expect(...)` to check expected conditions.
    let error1 = NSError(domain: "test", code: 1, userInfo: nil)
    let error2 = NSError(domain: "test", code: 2, userInfo: nil)
    #expect(Attempt<Never, EnrollError>.failure(.unknownError(error1)) == Attempt
      .failure(.unknownError(error1)))
    #expect(Attempt<Never, EnrollError>.failure(.unknownError(error1)) != Attempt
      .failure(.unknownError(error2)))
  }

  @Test func unexpectedPayload() async throws {
    let error = UnexpectedPayload(
      expected: "success",
      got: Optional(Teleport_Devicetrust_V1_EnrollDeviceResponse.OneOf_Payload
        .macosChallenge(Teleport_Devicetrust_V1_MacOSEnrollChallenge.with { _ in }))
    )

    #expect(error.localizedDescription == "expected success, got macosChallenge")
  }
}
