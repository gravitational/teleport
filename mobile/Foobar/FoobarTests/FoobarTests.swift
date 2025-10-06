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
  @Test func example() async throws {
    // Write your test here and use APIs like `#expect(...)` to check expected conditions.
    let error1 = NSError(domain: "test", code: 1, userInfo: nil)
    let error2 = NSError(domain: "test", code: 2, userInfo: nil)
    #expect(Attempt<Never, EnrollError>.failure(.unknownError(error1)) == Attempt
      .failure(.unknownError(error1)))
    #expect(Attempt<Never, EnrollError>.failure(.unknownError(error1)) != Attempt
      .failure(.unknownError(error2)))
  }
}
