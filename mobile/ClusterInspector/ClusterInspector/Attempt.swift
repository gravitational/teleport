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
//  Created by Rafał Cieślak on 2025-09-25.
//

public enum Attempt<
  Success: Equatable & Sendable,
  Failure: Error & Equatable & Sendable
>: Equatable,
  Sendable
{
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

  public var didFinish: Bool {
    switch self {
    case .failure, .success: true
    default: false
    }
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
