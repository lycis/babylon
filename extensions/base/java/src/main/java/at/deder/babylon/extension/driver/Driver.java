package at.deder.babylon.extension.driver;

import at.deder.babylon.extension.BabylonExtensionServer;
import at.deder.babylon.extension.Extension;
import io.vertx.core.Vertx;
import io.vertx.core.http.HttpResponseExpectation;
import io.vertx.core.json.JsonObject;
import io.vertx.ext.web.Router;
import io.vertx.ext.web.RoutingContext;
import io.vertx.ext.web.client.WebClient;
import io.vertx.ext.web.handler.BodyHandler;
import org.apache.logging.log4j.LogManager;
import org.apache.logging.log4j.Logger;

import java.net.MalformedURLException;
import java.net.URL;
import java.util.Map;
import java.util.concurrent.TimeUnit;

public abstract class Driver implements Extension {
  private String serverHostName;
  private int serverPort;
  private static Logger LOGGER = LogManager.getLogger();
  private int retryCounter = 0;
  private BabylonExtensionServer extensionServer;

  /**
     * Execute the given action with parameters.
     * @param action The driver action.
     * @param parameters Parameters for the action.
     * @return An ExecutionResult containing the outcome.
     */
    public abstract ExecutionResult execute(String action, Map<String, Object> parameters);

    /**
     * Returns the unique name of this driver.
     */
    public abstract String getName();

    /**
     * Returns the type of driver (e.g. "EBanking", "Logistics Application", etc.).
     */
    public abstract String getType();

  /**
   * Returns a shared secret between driver and server that may be to connect depending on the server config.
   */
    public abstract String getSecret();

  public void setupEndpoint(Router router) {
    Logger logger = LogManager.getLogger();
    router.post("/driver/"+getName().toLowerCase()+"/execute").handler(BodyHandler.create());
    router.post("/driver/"+getName().toLowerCase()+"/execute").handler(this::handleExecutionRequest);

    // server side registration endpoing
    router.post("/driver/"+getName().toLowerCase()+"/serverConnect").handler(BodyHandler.create());
    router.post("/driver/"+getName().toLowerCase()+"/serverConnect").handler(this::handleServerConnectRequest);

  }

  private void handleExecutionRequest(RoutingContext context) {
    Logger logger = LogManager.getLogger();
    JsonObject data = context.body().asJsonObject();
    String hostAddress = context.request().remoteAddress().hostAddress();
    String hostName = context.request().remoteAddress().hostName();

    if(data == null) {
      context.response().setStatusCode(400);
      context.json(new JsonObject().put("error", "missing payload"));
      logger.warn("Received invalid request. source=\"{} ({})\" reason=\"missing payloed\"", hostAddress,hostName);
      return;
    }

    if(!data.containsKey("action")) {
      context.response().setStatusCode(400);
      context.json(new JsonObject().put("error", "missing action"));
      logger.warn("Received invalid request. source=\"{} ({})\" reason=\"missing action\"", hostAddress,hostName);
      return;
    }

    if(!data.containsKey("session")) {
      context.response().setStatusCode(400);
      context.json(new JsonObject().put("error", "missing session id"));
      logger.warn("Received invalid request. source=\"{} ({})\" reason=\"missing session id\"", hostAddress,hostName);
      return;
    }

    String session = data.getString("session");
    String action = data.getString("action");
    Map<String, Object> parameters = null;

    logger.info("Received driver execution request. source=\"{} ({})\" session=\"{}\" action=\"{}\"", hostAddress,hostName,session,action);

    if(data.containsKey("parameters")) {
      parameters = data.getJsonObject("parameters").getMap();
    }

    ExecutionResult result = execute(action, parameters);

    context.json(result.toJson());
  }

  private void handleServerConnectRequest(RoutingContext context) {
    Logger logger = LogManager.getLogger();

    String hostAddress = context.request().remoteAddress().hostAddress();
    String hostName = context.request().remoteAddress().hostName();
    logger.info("Received server side registration request. source=\"{} ({})\"", hostAddress, hostName);

    JsonObject data = context.body().asJsonObject();
    if(data == null) {
      context.response().setStatusCode(400);
      context.json(new JsonObject().put("error", "missing payload"));
      logger.warn("Received invalid request. source=\"{} ({})\" reason=\"missing payload\"", hostAddress,hostName);
      return;
    }

    String callback = data.getString("callback");
    if(callback == null) {
      context.response().setStatusCode(400);
      context.json(new JsonObject().put("error", "missing callback"));
      logger.warn("Received invalid request. source=\"{} ({})\" reason=\"missing callback\"", hostAddress,hostName);
      return;
    }

    URL url = null;
    try {
      url = new URL(callback);
    } catch (MalformedURLException e) {
      context.response().setStatusCode(400);
      context.json(new JsonObject().put("error", "malformed callback URL"));
      logger.warn("Received invalid request. source=\"{} ({})\" reason=\"malformed callback URL\" url=\"{}\"", hostAddress,hostName,callback);
      return;
    }

    setRemoteServer( url.getHost(), url.getPort());

    // (String driver, String type, String callback
    var registrationData = new JsonObject()
      .put("driver", getName())
      .put("type", getType())
      .put("secret", getSecret())
      .put("callback", "http://"+extensionServer.getHostName()+":"+extensionServer.getPort()+"/");
    context.response().setStatusCode(200);
    context.json(registrationData);
    logger.info("Accepted server side driver registration");
  }

  @Override
  public void registerRemote(Vertx vertx) {
    var request = WebClient.create(vertx)
      .post(serverPort, serverHostName, "/driver/")
      .putHeader("Content-Type", "application/json");

    // (String driver, String type, String callback
    var registrationData = new JsonObject()
      .put("driver", getName())
      .put("type", getType());

    request.sendJsonObject(registrationData)
      .expecting(HttpResponseExpectation.SC_OK)
      .onSuccess(result -> LOGGER.info("Driver registered on remote server. remote=\"{}:{}\"", serverHostName, serverPort))
      .onFailure(e -> {
        if(retryCounter < 10) {
          LOGGER.info("Failed to register driver. Retrying. error=\"{}\"", e.getMessage());
          try {
            TimeUnit.SECONDS.sleep(5);
          } catch (InterruptedException ex) {
            throw new RuntimeException(ex);
          }
          retryCounter++;
          registerRemote(vertx);
        } else {
          LOGGER.fatal("Failed to register driver. Retries used up. error=\"{}\"", e.getMessage());
          throw new RuntimeException("Failed to register driver. Retries used up.");
        }
      });
  }

  @Override
  public void setRemoteServer(String serverHostName, int serverPort) {
    LogManager.getLogger().info("Updating remote server URL. host={} port={}", serverHostName, serverPort);
    this.serverHostName = serverHostName;
    this.serverPort = serverPort;
  }

  /**
   * If this returns true, then the driver will initiate a connection with the server. Return false here
   * if you are using server-side driver registration
   * @return <b>true</b> if the driver should self-register with the server
   */
  public boolean connectOnStartupEnabled() {
    return true;
  }

  public void setExtensionServer(BabylonExtensionServer extensionServer) {
    this.extensionServer = extensionServer;
  }
}

