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
	var iconScale: CGFloat
	var foreground: AnyShapeStyle
	var fillStyle: AnyShapeStyle
	var strokeStyle: AnyShapeStyle
	var maxWidth: CGFloat? = nil

	/// Initializes a standard-looking SF Symbol-based icon with various configurations.
	/// - Parameters:
	///   - systemName: The name of the SF Symbol
	///   - iconScale: The scale of the icon relative to its bounding box
	///   - foreground: The foreground style to use for the icon
	///   - fillStyle: The fill to use for the enclosing rounded rectangle
	///   - strokeStyle: The stroke to apply to the rounded rectangle. For no stroke, supply `.clear`.
	///   - maxWidth: The width (and therefore height) of the bounding box of the icon. Supply `nil` to allow the icon
	/// to grow to fill its space.
	init(
		systemName: String,
		iconScale: CGFloat = 0.45,
		foreground: some ShapeStyle = .foreground,
		fillStyle: some ShapeStyle = .background,
		strokeStyle: some ShapeStyle = .separator,
		maxWidth: CGFloat? = .xlarge * 2,
	) {
		self.systemName = systemName
		self.iconScale = iconScale
		self.foreground = AnyShapeStyle(foreground)
		self.fillStyle = AnyShapeStyle(fillStyle)
		self.strokeStyle = AnyShapeStyle(strokeStyle)
		self.maxWidth = maxWidth
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
		RoundedRectangle(cornerRadius: .medium)
			.fill(fillStyle)
			.strokeBorder(strokeStyle, lineWidth: 1)
			.aspectRatio(1, contentMode: .fit)
			.overlay {
				Image(systemName: systemName)
					.resizable()
					.aspectRatio(contentMode: .fill)
					.foregroundStyle(foreground)
					.frame(maxWidth: imageWidth, maxHeight: imageHeight)
			}
			.onGeometryChange(for: CGSize.self) { geometry in
				geometry.size
			} action: { size in
				self.size = size
			}
			.frame(maxWidth: maxWidth)
	}
}

#Preview("Basic Icon") {
	Icon(systemName: "viewfinder")
}

#Preview("App Tinted Icon") {
	Icon(
		systemName: "gearshape.fill",
		foreground: .tint,
		fillStyle: .tint.opacity(0.5),
		strokeStyle: .clear,
	)
}
