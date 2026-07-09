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

import Dependencies
import SwiftUI

struct PostEnrollmentLandingView: View {
	var clusters: [Cluster]
	var didTapOnCluster: (Cluster) async -> Void
	var didDeleteClustersAtIndex: (IndexSet) async -> Void

	var body: some View {
		List {
			Section {
				ForEach(clusters) { cluster in
					clusterView(for: cluster)
				}
				.onDelete { indexSet in
					Task { await didDeleteClustersAtIndex(indexSet) }
				}
			} header: {
				Text("Clusters")
			}
		}
		.padding(.horizontal)
		.background(Color.Background.depth3)
		.navigationTitle("Teleport")
	}
}

extension PostEnrollmentLandingView {
	private func clusterView(for cluster: Cluster) -> some View {
		Button {
			Task { await didTapOnCluster(cluster) }
		} label: {
			HStack(alignment: .firstTextBaseline) {
				Text(cluster.host)
					.frame(maxWidth: .infinity, alignment: .leading)
				Image(systemName: "arrow.up.right.square")
			}
		}
	}
}

#Preview("Post Enrollment") {
	@Previewable @State
	var clusters = [
		Cluster(id: UUID(), host: "production.teleport.sh", port: 443),
		Cluster(id: UUID(), host: "stating.teleport.sh", port: 8080),
		Cluster(id: UUID(), host: "dev.teleport.sh", port: 2048),
	]

	NavigationStack {
		PostEnrollmentLandingView(
			clusters: clusters,
			didTapOnCluster: { print("User tapped on \($0.host)") },
			didDeleteClustersAtIndex: { clusters.remove(atOffsets: $0) },
		)
	}
}
