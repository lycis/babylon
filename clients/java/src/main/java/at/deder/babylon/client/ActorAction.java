package at.deder.babylon.client;

import java.util.HashMap;
import java.util.Map;

public class ActorAction {
    private final Map<String, Object> parameters = new HashMap<String, Object>();
    private final String driverType;
    private final BabylonClient client;
    private String action;

    public ActorAction(BabylonClient client, String driverType) {
        this.client = client;
        this.driverType = driverType;
    }


    public String getSession() {
        return client.session().uuid();
    }

    public String getType() {
        return driverType;
    }

    public String getAction() {
        return action;
    }

    public ActorAction action(String action) {
        this.action = action;
        return this;
    }

    public Map<String, Object> getParameters() {
        return parameters;
    }

    public ActorAction parameter(String name, Object value) {
        parameters.put(name, value);
        return this;
    }

    public ExecutionResult execute() {
        return client.api().executeActorAction(this);
    }
}
