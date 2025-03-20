package at.deder.babylon.examples.client;

import at.deder.babylon.client.BabylonClient;
import at.deder.babylon.client.Session;
import at.deder.babylon.client.SessionLogMessage;

public class Main {
    public static void main(String... args) {
        var api = BabylonClient.createFor("http://localhost:8080/");
        Session session = api.session();
        System.out.println("New session: "+session.uuid());
        var result = api.driver("example").action("exampleAction").parameter("a", 4711).execute();
        System.out.println("action result = "+result.success()+" - "+result.message());
        System.out.println(api.refreshSessionInfo().context().log()
                .stream()
                .map(log -> log.timestamp() + "-" + log.type() + "-" + log.message())
                .reduce("", (s,n) -> s+"\n"+n));
    }
}
