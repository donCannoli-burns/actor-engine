import Foundation
#if canImport(FoundationNetworking)
import FoundationNetworking
#endif

let sem = DispatchSemaphore(value: 0)
let url = URL(string: "http://127.0.0.1:10424/v1/state")!
URLSession.shared.dataTask(with: url) { data, _, error in
    defer { sem.signal() }
    if let error { fputs("\(error)\n", stderr); return }
    if let data, let text = String(data: data, encoding: .utf8) { print(text) }
}.resume()
sem.wait()
