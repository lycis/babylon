package at.deder.babylon.extension;

import io.vertx.core.Vertx;
import io.vertx.ext.web.Router;

public interface Extension {
  // Set up the endpoint for this extension in the router
  void setupEndpoint(Router router);

  void registerRemote(Vertx vertx);

  void setRemoteServer(String serverHostName, int serverPort);

  boolean connectOnStartupEnabled();

  void setExtensionServer(BabylonExtensionServer extensionServer);
}
