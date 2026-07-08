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

import SwiftUI

struct PostEnrollmentLandingView: View {
	var clusters: [Cluster]

	var body: some View {
		ScrollView {
			LazyVStack(spacing: .large) {
				clusterList
			}
		}
		.padding(.horizontal)
		.background(Color.Background.depth3)
	}
}

extension PostEnrollmentLandingView {
	private var clusterList: some View {
		ForEach(clusters, id: \.id) { cluster in
			Label {
				Text(cluster.host)
			} icon: {
				Image(systemName: "server.rack")
			}
			.frame(maxWidth: .infinity, alignment: .leading)
			.padding()
			.background(
				RoundedRectangle(cornerRadius: .small)
					.fill(Color.Background.depth2),
			)
		}
	}
}

#Preview("Post Enrollment") {
	PostEnrollmentLandingView(
		clusters: [
			Cluster(id: UUID(), host: "example.teleport.sh"),
		],
	)
}
