// KoL Actor Engine observation shim.
// This ASH file does not execute arbitrary commands from the actor engine.
// It only publishes a bounded runtime snapshot to the local Go supervisor.

string KOL_ACTOR_URL = "http://127.0.0.1:10424";

boolean kol_actor_reachable() {
    buffer response = visit_url(KOL_ACTOR_URL + "/health", false);
    return contains_text(response.to_string(), "\"ok\":true");
}

void kol_actor_sync(string event_name) {
    string url = KOL_ACTOR_URL + "/v1/kolmafia/update"
        + "?event=" + url_encode(event_name)
        + "&character=" + url_encode(my_name())
        + "&total_turns=" + to_string(total_turns_played())
        + "&ascension_turns=" + to_string(my_turncount())
        + "&adventures=" + to_string(my_adventures())
        + "&ascensions=" + to_string(my_ascensions())
        + "&breakfast=" + get_property("breakfastCompleted");
    buffer response = visit_url(url, false, true);
    if (!contains_text(response.to_string(), "\"accepted\":true")) {
        print("KoL Actor Engine did not accept the observation.", "red");
    }
}

void kol_actor_status() {
    if (kol_actor_reachable()) {
        print("KoL Actor Engine reachable at " + KOL_ACTOR_URL, "navy");
        print("ASH execution authority: false", "navy");
        return;
    }
    print("KoL Actor Engine not reachable at " + KOL_ACTOR_URL, "red");
}

void main(string command) {
    string cmd = to_lower_case(command);
    if (cmd == "" || cmd == "status") {
        kol_actor_status();
        return;
    }
    if (cmd == "sync") {
        kol_actor_sync("manual");
        return;
    }
    if (cmd == "login") {
        kol_actor_sync("login");
        return;
    }
    if (cmd == "turn") {
        kol_actor_sync("after-adventure");
        return;
    }
    print("Usage: kol_actor [status|sync|login|turn]", "red");
}
