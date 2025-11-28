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
//  ContentView.swift
//  ClusterInspector
//
//  Created by Rafał Cieślak on 2025-11-28.
//

import Ping
import SwiftUI

struct ContentView: View {
  @State private var clusterAddress = ""
  @FocusState private var isFocused: Bool

  var body: some View {
    VStack(spacing: 16) {
      VStack {
        Image("logo").resizable().aspectRatio(contentMode: .fit)
          .frame(height: 60)
      }
      Spacer()

      TextField("Cluster Address", text: $clusterAddress).autocorrectionDisabled(true)
        .keyboardType(.URL).textInputAutocapitalization(.never).focused($isFocused)
        .submitLabel(.send).textFieldStyle(.roundedBorder)
        .modifier(ClearButton(text: $clusterAddress)).onSubmit {
          Task {
            let finder = PingFinder()!
            do {
              let response = try finder.find(clusterAddress)
              print(response)
            } catch {
              print(error)
            }
          }
        }
    }.padding(16)
  }
}

#Preview {
  ContentView()
}

struct ClearButton: ViewModifier {
  @Binding var text: String

  func body(content: Content) -> some View {
    HStack {
      content

      if !text.isEmpty {
        Button(action: {
          text = ""
        }) {
          Image(systemName: "xmark.circle.fill")
            .foregroundColor(Color(UIColor.opaqueSeparator))
            .padding(.trailing, 8)
            .padding(.vertical, 8)
            .contentShape(Rectangle())
        }
      }
    }
  }
}
