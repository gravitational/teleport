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
