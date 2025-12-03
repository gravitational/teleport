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
//  ClusterInspectorTests.swift
//  ClusterInspectorTests
//
//  Created by Rafał Cieślak on 2025-11-28.
//

@testable import ClusterInspector
import Testing

// @MainActor needs to be added, otherwise there are weird errors regarding actor isolation.
// It might be because FindViewModel is @Observable or because it holds PingFindResponse in its
// state.
@MainActor struct FindViewModelTests {
  @Test func proxyServerValid() {
    let m = FindViewModel()
    m.clusterAddress = "teleport.example.com:3080"
    #expect(m.proxyServer == "teleport.example.com:3080")
  }

  @Test func proxyServerMissingPort() {
    let m = FindViewModel()
    m.clusterAddress = "teleport.example.com"
    #expect(m.proxyServer == "teleport.example.com:443")
  }

  @Test func proxyServerWithScheme() {
    let m = FindViewModel()
    m.clusterAddress = "https://teleport.example.com:3080"
    #expect(m.proxyServer == "teleport.example.com:3080")
  }

  @Test func proxyServerWithSchemeAndMissingPort() {
    let m = FindViewModel()
    m.clusterAddress = "https://teleport.example.com"
    #expect(m.proxyServer == "teleport.example.com:443")
  }

  @Test func proxyServerWithPath() {
    let m = FindViewModel()
    m.clusterAddress = "https://teleport.example.com/web"
    #expect(m.proxyServer == "teleport.example.com:443")
  }

  @Test func proxyServerWithPathAndPort() {
    let m = FindViewModel()
    m.clusterAddress = "https://teleport.example.com:3080/web"
    #expect(m.proxyServer == "teleport.example.com:3080")
  }
}
