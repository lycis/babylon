package at.deder.babylon.extension;

import io.vertx.core.AbstractVerticle;
import io.vertx.core.DeploymentOptions;
import io.vertx.core.Promise;
import io.vertx.core.Vertx;
import io.vertx.ext.web.Router;
import org.apache.logging.log4j.LogManager;
import org.apache.logging.log4j.Logger;

import java.util.ArrayList;
import java.util.List;

public class BabylonExtensionServer extends AbstractVerticle {
  private String serverHostName;
  private int serverPort;
  private final List<Extension> extensions = new ArrayList<>();
  private int port = 8888;
  private static final Logger LOGGER = LogManager.getLogger();

  private BabylonExtensionServer() {
    this.serverHostName = "localhost";
    this.serverPort = 8080;
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

    updateExtensionServer();
  }

  private void updateExtensionServer() {
    extensions.forEach(ext ->ext.setExtensionServer(this));
  }

  public BabylonExtensionServer registerWithRemoteServer(String serverHostName, int serverPort) {
    this.serverHostName = serverHostName;
    this.serverPort = serverPort;
    extensions.forEach(ext -> ext.registerRemote(vertx));
    return this;
  }

  private void addExtension(Extension ext) {
    extensions.add(ext);
  }

  public int getPort() {
    return port;
  }

  public BabylonExtensionServer setPort(int port) {
    this.port = port;
    return this;
  }

  public String getHostName() {
    return "localhost";
  }

  public static BabylonExtensionServer forActor(ExecutableExtension implementation) {
    var server = new BabylonExtensionServer();
    server.addExtension(new Executor(ExtensionType.ACTOR, implementation));
    return server;
  }

  public static BabylonExtensionServer forDriver(ExecutableExtension implementation) {
    var server = new BabylonExtensionServer();
    server.addExtension(new Executor(ExtensionType.DRIVER, implementation));
    return server;
  }

  public static BabylonExtensionServer forReporter(ReporterExtension implementation) {
    var server = new BabylonExtensionServer();
    server.addExtension(new Reporter(implementation));
    return server;
  }


  public void run() {
    Vertx vertx = Vertx.vertx();

    vertx.deployVerticle(this, new DeploymentOptions()
      .setWorkerPoolSize(10)
      .setWorkerPoolName("server"));
  }
}
