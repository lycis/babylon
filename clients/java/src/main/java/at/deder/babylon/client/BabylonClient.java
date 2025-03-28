package at.deder.babylon.client;

import feign.Feign;
import feign.jackson.JacksonDecoder;
import feign.jackson.JacksonEncoder;

public class BabylonClient {
    private BabylonClientAPI api;
    private Session currentSession;

    private BabylonClient() {
    }

    public synchronized static BabylonClient createFor(String url) {
        var bc = new BabylonClient();
        bc.api = Feign.builder()
                .decoder(new JacksonDecoder())
                .encoder(new JacksonEncoder())
                .target(BabylonClientAPI.class, url);
        return bc;
    }

    public synchronized Session session() {
        if(currentSession == null) {
            currentSession = api.createNewSession();
        }
        return currentSession;
    }

    public DriverAction driver(String driver) {
        return new DriverAction(this, driver);
    }

    public void endSession() {
        if(currentSession == null) {
            throw new IllegalStateException("no active session");
        }
        api.endSession(currentSession.uuid());
        currentSession = null;
    }

    public BabylonClientAPI api() {
        return api;
    }

    public synchronized void reuseSession(String sessionId) {
        currentSession = api.sessionInfo(sessionId);
    }

    public synchronized Session refreshSessionInfo() {
        currentSession = api.sessionInfo(session().uuid());
        return currentSession;
    }

    public ActorAction actor(String name) {
        return new ActorAction(this, name);
    }
}
