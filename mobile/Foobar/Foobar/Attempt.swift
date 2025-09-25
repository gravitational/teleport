//
//  Attempt.swift
//  Foobar
//
//  Created by Rafał Cieślak on 2025-09-25.
//

public enum Attempt<Success: Sendable, Failure: Error & Sendable>: Sendable {
  case idle
  case loading
  case success(Success)
  case failure(Failure)

  public var didFail: Bool {
    if case .failure = self { return true }
    return false
  }

  public var didSucceed: Bool {
    if case .success = self { return true }
    return false
  }

  public var isLoading: Bool {
    if case .loading = self { return true }
    return false
  }

  public var isIdle: Bool {
    if case .idle = self { return true }
    return false
  }

  func map<U: Equatable & Sendable>(_ transform: (Success) -> U) -> Attempt<U, Failure> {
    switch self {
    case .idle: .idle
    case .loading: .loading
    case let .success(value): .success(transform(value))
    case let .failure(error): .failure(error)
    }
  }
}
