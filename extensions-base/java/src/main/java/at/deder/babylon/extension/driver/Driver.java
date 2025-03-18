package at.deder.babylon.extension.driver;

import at.deder.babylon.extension.Extension;
import io.vertx.core.Vertx;
import io.vertx.core.http.HttpResponseExpectation;
import io.vertx.core.json.Json;
import io.vertx.core.json.JsonObject;
import io.vertx.ext.web.Router;
import io.vertx.ext.web.client.WebClient;
import io.vertx.ext.web.handler.BodyHandler;
import org.apache.logging.log4j.LogManager;
import org.apache.logging.log4j.Logger;

import java.util.Map;
import java.util.concurrent.TimeUnit;

public abstract class Driver implements Extension {
  private String serverHostName;
  private int serverPort;
  private static Logger LOGGER = LogManager.getLogger();
  private int retryCounter = 0;

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

  public void setupEndpoint(Router router) {
    Logger logger = LogManager.getLogger();
    router.post("/driverExecute").handler(BodyHandler.create());
    router.post("/driverExecute").handler(context -> {

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

      String action = data.getString("action");
      Map<String, Object> parameters = null;

      logger.info("Received driver exection request. source=\"{} ({})\" action=\"{}\"", hostAddress,hostName,action);

      if(data.containsKey("parameters")) {
        parameters = data.getJsonObject("parameters").getMap();
      }

      ExecutionResult result = execute(action, parameters);

      context.json(result.toJson());
    });
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
          LOGGER.fatal("Failed to register driver. Retreies used up. error=\"{}\"", e.getMessage());
          throw new RuntimeException("Failed to register driver. Retreies used up.");
        }
      });
  }

  @Override
  public void setRemoteServer(String serverHostName, int serverPort) {
    this.serverHostName = serverHostName;
    this.serverPort = serverPort;
  }
}
