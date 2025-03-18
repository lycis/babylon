package at.deder.babylon.extension.example;

import at.deder.babylon.extension.BabylonExtensionServer;

public class ExampleExtensionServer extends BabylonExtensionServer {
  public ExampleExtensionServer() {
    super("localhost", 8080);
    addExtension(new ExampleDriver());
    setPort(8082);
  }
}
