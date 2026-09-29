import java.net.URI
import java.net.http.HttpClient
import java.net.http.HttpRequest
import java.net.http.HttpResponse

fun main() {
    val client = HttpClient.newHttpClient()
    val request = HttpRequest.newBuilder(URI.create("http://127.0.0.1:10424/v1/state")).GET().build()
    val response = client.send(request, HttpResponse.BodyHandlers.ofString())
    println(response.body())
}
