package at.deder.babylon.examples.extensions;

import at.deder.babylon.client.BabylonClient;
import at.deder.babylon.extension.BabylonExtensionServer;
import at.deder.babylon.extension.ExecutableExtension;
import at.deder.babylon.extension.ExecutionResult;

import java.util.Map;

public class ExampleActor implements ExecutableExtension {

    public static void main(String... args) {
        BabylonExtensionServer
                .forActor(new ExampleActor())
                .setPort(9093)
                .run();
    }


    @Override
    public ExecutionResult execute(String action, Map<String, Object> parameters, BabylonClient api) {
        return new ExecutionResult(true, "action executed with parameters: "+parameters);
    }

    @Override
    public String getName() {
        return "exampleactor";
    }

    @Override
    public String getType() {
        return "example";
    }

    @Override
    public String getSecret() {
        return "exampleSecretActor";
    }

    @Override
    public boolean connectOnStartupEnabled() {
        return false;
    }
}
