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
// along with this program.  If not, see http://www.gnu.org/licenses/

import Dispatch
import Synchronization

/// Coordinates an asynchronous test with a synchronous worker by pausing the worker's next call.
///
/// Each gate blocks one call. Create another gate when a test needs to coordinate an additional call.
///
/// ```swift
/// let gate = BlockingCallGate()
///
/// workerQueue.async {
/// 	gate.blockNextCall()
/// 	performSynchronousWork()
/// }
///
/// try await gate.withNextCallBlocked {
/// 	// Inspect or modify state while the worker is paused.
/// }
/// ```
final class BlockingCallGate: Sendable {
	private let callReachedGate = DispatchSemaphore(value: 0)
	private let callMayProceed = DispatchSemaphore(value: 0)
	private let shouldBlockNextCall = Mutex(true)

	/// Blocks the calling worker thread on the next invocation and lets subsequent calls proceed normally.
	func blockNextCall() {
		let shouldBlock = shouldBlockNextCall.withLock { shouldBlock in
			defer { shouldBlock = false }
			return shouldBlock
		}
		guard shouldBlock else { return }

		callReachedGate.signal()
		callMayProceed.wait()
	}

	/// Runs an operation while the next call is blocked, then always releases the blocked worker.
	func withNextCallBlocked(_ operation: () -> Void) async throws {
		let waitResult = await withCheckedContinuation { continuation in
			DispatchQueue.global().async {
				continuation.resume(returning: self.callReachedGate.wait(timeout: .now() + .seconds(5)))
			}
		}
		defer { callMayProceed.signal() }

		guard waitResult == .success else {
			throw BlockingCallGateError.timedOutWaitingForNextCall
		}
		operation()
	}
}

private enum BlockingCallGateError: Error {
	case timedOutWaitingForNextCall
}
