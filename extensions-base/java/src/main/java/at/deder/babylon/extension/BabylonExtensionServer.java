package at.deder.babylon.extension;

import io.vertx.core.AbstractVerticle;
import io.vertx.core.Promise;
import io.vertx.ext.web.Router;
import org.apache.logging.log4j.LogManager;
import org.apache.logging.log4j.Logger;

import java.util.ArrayList;
import java.util.List;

public abstract class BabylonExtensionServer extends AbstractVerticle {
  private final String serverHostName;
  private final int serverPort;
  private final List<Extension> extensions = new ArrayList<>();
  private int port = 8888;
  private static final Logger LOGGER = LogManager.getLogger();

  public BabylonExtensionServer(String serverHostName, int port) {
    this.serverHostName = serverHostName;
    this.serverPort = port;
  }

  @Override
  public void start(Promise<Void> startPromise) {
    Router router = Router.router(vertx);

    // enable endpoints for all known extenions
    extensions.forEach(ext -> {
      ext.setRemoteServer(serverHostName, serverPort);
      ext.setupEndpoint(router);
    });

    // Create the HTTP server
    vertx.createHttpServer()
      // Handle every request using the router
      .requestHandler(router)
      // Start listening
      .listen(getPort())
      // Print the port on success
      .onSuccess(server -> {
        LOGGER.info("HTTP server started on port {}", server.actualPort());
        startPromise.complete();
      })
      // Print the problem on failure
      .onFailure(throwable -> {
        LOGGER.error("HTTP Server errored", throwable);
        startPromise.fail(throwable);
      });

    registerWithRemoteServer();
  }

  private void registerWithRemoteServer() {
    extensions.forEach(ext -> ext.registerRemote(vertx));
  }

  public void addExtension(Extension ext) {
    extensions.add(ext);
  }

  public int getPort() {
    return port;
  }

  public void setPort(int port) {
    this.port = port;
  }
}
