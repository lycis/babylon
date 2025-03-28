package at.deder.babylon.examples.extensions;


import at.deder.babylon.client.BabylonClient;
import at.deder.babylon.extension.BabylonExtensionServer;
import at.deder.babylon.extension.ExecutableExtension;
import at.deder.babylon.extension.ExecutionResult;

import java.util.Map;

public class ExampleDriver  implements ExecutableExtension {

  public static void main(String... args) {
   BabylonExtensionServer
           .forDriver(new ExampleDriver())
           .setPort(9092)
           .run();
  }

  @Override
  public ExecutionResult execute(String action, Map<String, Object> parameters, BabylonClient api) {
    // Implement your driver-specific action logic here.
    api.actor("example").action("doSomething").parameter("test", 1234).execute();
    String message = "Executed action '" + action + "' with parameters: " + parameters;
    System.out.println(message);
    return new ExecutionResult(true, message);
  }

  @Override
  public String getName() {
    return "examplejavadriver";
  }

  @Override
  public String getType() {
    return "example";
  }

  @Override
  public String getSecret() {
    return "someTestSecret";
  }

  @Override
  public boolean connectOnStartupEnabled() {
    return false;
  }
}
