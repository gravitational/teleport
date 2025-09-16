//
//  ContentView.swift
//  Foobar
//
//  Created by Rafał Cieślak on 2025-09-11.
//

import SwiftUI

struct ContentView: View {
  @State private var proxyAddr: String = "cluster.mirrors.link"
  @FocusState private var focused: Bool

  var body: some View {
    VStack(spacing: 16) {
      VStack {
        Image("logo")
          .resizable()
          .aspectRatio(contentMode: .fit)
          .padding(.horizontal, 16)
          .frame(height: 60)
      }.padding(.top, 16)
      Form {
        TextField("Cluster Address", text: $proxyAddr)
          .focused($focused)
          .autocorrectionDisabled()
          .textInputAutocapitalization(.never)
          .keyboardType(.URL)
          .textContentType(.URL)
          .onAppear {
            UITextField.appearance().clearButtonMode = .whileEditing
          }
      }.onAppear {
        focused = true
      }
      .onSubmit {
        var addr = proxyAddr
        if !addr.starts(with: "https://") {
          addr = "https://\(addr)"
        }
        // TODO(ravicious): Ping the cluster first to see if it's a Teleport cluster.
        var url = URL(string: addr)!
        url.append(components: "webapi", "mfa", "login", "begin")
        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.httpBody = Data("{\"passwordless\":true}".utf8)

        Task {
          var data: Data
          var response: URLResponse
          do {
            (data, response) = try await URLSession.shared.data(for: request)
          } catch {
            print("Failed to send POST request: \(error)")
            return
          }
          guard let httpResponse = response as? HTTPURLResponse,
                (200 ... 299).contains(httpResponse.statusCode)
          else {
            print("Got non-200 response: \(response)")
            return
          }
          let decoder = JSONDecoder()
          var mfaResponse: WebapiMFALoginBeginResponse
          do {
            mfaResponse = try decoder.decode(WebapiMFALoginBeginResponse.self, from: data)
          } catch {
            print("Failed to decode response: \(error)")
            return
          }
          print("Got response \(mfaResponse)")
        }
      }.submitLabel(.send)
    }
  }
}

#Preview {
  ContentView()
}

struct WebapiMFALoginBeginResponse: Codable {
  let webauthnChallenge: WebauthnChallenge

  enum CodingKeys: String, CodingKey {
    case webauthnChallenge = "webauthn_challenge"
  }
}

struct WebauthnChallenge: Codable {
  let publicKey: PublicKey
}

struct PublicKey: Codable {
  let challenge: String
  let timeout: Int
  let rpID, userVerification: String

  enum CodingKeys: String, CodingKey {
    case challenge, timeout
    case rpID = "rpId"
    case userVerification
  }
}
