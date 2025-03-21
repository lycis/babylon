package at.deder.babylon.examples.extensions;


import at.deder.babylon.extension.BabylonExtensionServer;
import at.deder.babylon.extension.driver.Driver;
import at.deder.babylon.extension.driver.ExecutionResult;

import java.util.Map;

public class ExampleDriver  extends Driver {

  @Override
  public ExecutionResult execute(String action, Map<String, Object> parameters) {
    // Implement your driver-specific action logic here.
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
