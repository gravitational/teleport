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

import SwiftUI

/// A general-purpose SF-symbol based icon boxed into a rounded rect.
struct Icon: View {
	// MARK: Initializer

	let systemName: String
	var iconScale = 0.5
	var foreground: AnyShapeStyle
	var fillStyle: AnyShapeStyle
	var strokeStyle: AnyShapeStyle

	init(
		systemName: String,
		iconScale: Double = 0.5,
		foreground: some ShapeStyle = .foreground,
		fillStyle: some ShapeStyle = .background,
		strokeStyle: some ShapeStyle = .separator,
	) {
		self.systemName = systemName
		self.iconScale = iconScale
		self.foreground = AnyShapeStyle(foreground)
		self.fillStyle = AnyShapeStyle(fillStyle)
		self.strokeStyle = AnyShapeStyle(strokeStyle)
	}

	// MARK: State

	@State
	private var size: CGSize = .zero

	// MARK: UI Helpers

	private var imageWidth: CGFloat {
		size.width * iconScale
	}

	private var imageHeight: CGFloat {
		size.height * iconScale
	}

	// MARK: Body

	var body: some View {
		RoundedRectangle(cornerRadius: 8)
			.fill(fillStyle)
			.strokeBorder(strokeStyle)
			.aspectRatio(1, contentMode: .fit)
			.overlay {
				Image(systemName: systemName)
					.resizable()
					.foregroundStyle(foreground)
					.frame(maxWidth: imageWidth, maxHeight: imageHeight)
			}
			.onGeometryChange(for: CGSize.self) { geometry in
				geometry.size
			} action: { size in
				self.size = size
			}
	}
}

#Preview("Basic Icon") {
	Icon(systemName: "viewfinder")
		.frame(width: 64, height: 64)
}

#Preview("App Tinted Icon") {
	Icon(
		systemName: "gearshape.fill",
		foreground: .tint,
		fillStyle: .tint.opacity(0.5),
		strokeStyle: .clear,
	)
	.frame(width: 64, height: 64)
}
