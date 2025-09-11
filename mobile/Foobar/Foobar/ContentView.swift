//
//  ContentView.swift
//  Foobar
//
//  Created by Rafał Cieślak on 2025-09-11.
//

import SwiftUI

struct ContentView: View {
    var body: some View {
        VStack {
            Image("logo").resizable().aspectRatio(contentMode: .fit)

            Text("Hello, world!")
                .font(.title)
        }
        .padding(16)
    }
}

#Preview {
    ContentView()
}
