using System.Net.Http;

using var client = new HttpClient();
Console.WriteLine(await client.GetStringAsync("http://127.0.0.1:10424/v1/state"));
