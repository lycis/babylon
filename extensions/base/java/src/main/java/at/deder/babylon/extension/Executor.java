package at.deder.babylon.extension;

import at.deder.babylon.client.BabylonClient;
import io.vertx.core.Vertx;
import io.vertx.core.http.HttpResponseExpectation;
import io.vertx.core.json.JsonObject;
import io.vertx.ext.web.Router;
import io.vertx.ext.web.RoutingContext;
import io.vertx.ext.web.client.WebClient;
import io.vertx.ext.web.handler.BodyHandler;
import io.vertx.ext.web.impl.BlockingHandlerDecorator;
import org.apache.logging.log4j.LogManager;
import org.apache.logging.log4j.Logger;

import java.net.MalformedURLException;
import java.net.URL;
import java.util.Map;
import java.util.concurrent.TimeUnit;

public class Executor extends ExtensionBase implements Extension {
  private final ExtensionType type;
  private final ExecutableExtension implementation;
  String serverHostName;
  int serverPort;
  static final Logger LOGGER = LogManager.getLogger();
  int retryCounter;
  BabylonExtensionServer extensionServer;

  public Executor(ExtensionType type, ExecutableExtension implementation) {
    this.type = type;
    this.implementation = implementation;
  }

  public void setupEndpoint(Router router) {
    Logger logger = LogManager.getLogger();
    router.post("/"+ lowerCaseCategory() +"/"+ getImplementation().getName().toLowerCase()+"/execute").handler(BodyHandler.create());
    router.post("/"+ lowerCaseCategory() +"/"+ getImplementation().getName().toLowerCase()+"/execute").handler(new BlockingHandlerDecorator(this::handleExecutionRequest, true));

    // server side registration endpoint
    router.post("/"+ lowerCaseCategory() +"/"+ getImplementation().getName().toLowerCase()+"/serverConnect").handler(BodyHandler.create());
    router.post("/"+ lowerCaseCategory() +"/"+ getImplementation().getName().toLowerCase()+"/serverConnect").handler(new BlockingHandlerDecorator(this::handleServerConnectRequest, true));

    // session end endpoint
    router.delete("/"+ lowerCaseCategory() +"/"+ getImplementation().getName().toLowerCase()+"/session/:id").handler(new BlockingHandlerDecorator(this::handleSessionEnd, true));
  }

  private void handleSessionEnd(RoutingContext routingContext) {
    String id = routingContext.pathParam("id");
    getImplementation().onSessionEnd(id);
    routingContext.json("ok");
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

    logger.info("Received {} execution request. source=\"{} ({})\" session=\"{}\" action=\"{}\"", lowerCaseCategory(), hostAddress, hostName, session, action);

    if(data.containsKey("parameters")) {
      parameters = data.getJsonObject("parameters").getMap();
    }

    ExecutionResult result = getImplementation().execute(action, parameters, createBabylonClient(session));

    context.json(result.toJson());
  }

  private BabylonClient createBabylonClient(String sessionId) {
    var client = BabylonClient.createFor("http://" + serverHostName + ":" + serverPort + "/");
    client.reuseSession(sessionId);
    return client;
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
      .put("name", getImplementation().getName())
      .put("type", getImplementation().getType())
      .put("secret", getImplementation().getSecret())
      .put("callback", "http://"+extensionServer.getHostName()+":"+extensionServer.getPort()+"/");
    context.response().setStatusCode(200);
    context.json(registrationData);
    logger.info("Accepted server side "+ lowerCaseCategory() +" registration");
  }

  private ExecutableExtension getImplementation() {
    return implementation;
  }

  @Override
  public void registerRemote(Vertx vertx) {
    var request = WebClient.create(vertx)
      .post(serverPort, serverHostName, "/"+ lowerCaseCategory() +"/")
      .putHeader("Content-Type", "application/json");

    // (String driver, String type, String callback
    var registrationData = new JsonObject()
      .put(lowerCaseCategory(), getImplementation().getName())
      .put("type", getImplementation().getType());

    request.sendJsonObject(registrationData)
      .expecting(HttpResponseExpectation.SC_OK)
      .onSuccess(result -> LOGGER.info("{} registered on remote server. remote=\"{}:{}\"", lowerCaseCategory(), serverHostName, serverPort))
      .onFailure(e -> {
        if(retryCounter < 10) {
          LOGGER.info("Failed to register "+ lowerCaseCategory() +". Retrying. error=\"{}\"", e.getMessage());
          try {
            TimeUnit.SECONDS.sleep(5);
          } catch (InterruptedException ex) {
            throw new RuntimeException(ex);
          }
          retryCounter++;
          registerRemote(vertx);
        } else {
          LOGGER.fatal("Failed to register "+ lowerCaseCategory() +". Retries used up. error=\"{}\"", e.getMessage());
          throw new RuntimeException("Failed to register "+ lowerCaseCategory() +". Retries used up.");
        }
      });
  }

  private String lowerCaseCategory() {
    return type.name().toLowerCase();
  }

  @Override
  public void setRemoteServer(String serverHostName, int serverPort) {
    LogManager.getLogger().info("Updating remote server URL. host={} port={}", serverHostName, serverPort);
    this.serverHostName = serverHostName;
    this.serverPort = serverPort;
  }

  @Override
  public void setExtensionServer(BabylonExtensionServer extensionServer) {
    this.extensionServer = extensionServer;
  }

}
