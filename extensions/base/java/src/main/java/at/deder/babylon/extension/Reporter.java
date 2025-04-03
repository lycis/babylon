package at.deder.babylon.extension;

import at.deder.babylon.client.Session;
import at.deder.babylon.client.SessionContext;
import at.deder.babylon.client.SessionLogMessage;
import io.vertx.core.Vertx;
import io.vertx.core.json.JsonArray;
import io.vertx.core.json.JsonObject;
import io.vertx.ext.web.Router;
import io.vertx.ext.web.RoutingContext;
import io.vertx.ext.web.handler.BodyHandler;
import io.vertx.ext.web.impl.BlockingHandlerDecorator;

import org.apache.logging.log4j.LogManager;
import org.apache.logging.log4j.Logger;

import java.net.MalformedURLException;
import java.net.URL;
import java.time.OffsetDateTime;
import java.util.ArrayList;
import java.util.Date;
import java.util.List;

public class Reporter implements Extension {
  private static final Logger LOGGER = LogManager.getLogger();
  private final ReporterExtension implementation;
  private BabylonExtensionServer extensionServer;

  public Reporter(ReporterExtension implementation) {
    this.implementation = implementation;
  }

  @Override
  public void setupEndpoint(Router router) {
    LOGGER.info("Setting up reporter endpoint. name={}", implementation.getName());

    // live logging endpoint
    if (implementation.isLiveReporter()) {
      router.post("/reporter/" + implementation.getName().toLowerCase() + "/live").handler(BodyHandler.create());
      router.post("/reporter/" + implementation.getName().toLowerCase() + "/live").handler(new BlockingHandlerDecorator(this::handleLiveReport, true));
    }

    // session end report endpoint
    router.post("/reporter/" + implementation.getName().toLowerCase() + "/report").handler(BodyHandler.create());
    router.post("/reporter/" + implementation.getName().toLowerCase() + "/report").handler(new BlockingHandlerDecorator(this::handleEndReport, true));


    // server side registration endpoint
    router.post("/reporter/" + implementation.getName().toLowerCase() + "/serverConnect").handler(BodyHandler.create());
    router.post("/reporter/" + implementation.getName().toLowerCase() + "/serverConnect").handler(new BlockingHandlerDecorator(this::handleServerConnectRequest, true));
  }

  private void handleServerConnectRequest(RoutingContext context) {
    Logger logger = LogManager.getLogger();

    String hostAddress = context.request().remoteAddress().hostAddress();
    String hostName = context.request().remoteAddress().hostName();
    logger.info("Received server side registration request. source=\"{} ({})\"", hostAddress, hostName);

    JsonObject data = context.body().asJsonObject();
    if (data == null) {
      context.response().setStatusCode(400);
      context.json(new JsonObject().put("error", "missing payload"));
      logger.warn("Received invalid request. source=\"{} ({})\" reason=\"missing payload\"", hostAddress, hostName);
      return;
    }

    String callback = data.getString("callback");
    if (callback == null) {
      context.response().setStatusCode(400);
      context.json(new JsonObject().put("error", "missing callback"));
      logger.warn("Received invalid request. source=\"{} ({})\" reason=\"missing callback\"", hostAddress, hostName);
      return;
    }

    URL url = null;
    try {
      url = new URL(callback);
    } catch (MalformedURLException e) {
      context.response().setStatusCode(400);
      context.json(new JsonObject().put("error", "malformed callback URL"));
      logger.warn("Received invalid request. source=\"{} ({})\" reason=\"malformed callback URL\" url=\"{}\"", hostAddress, hostName, callback);
      return;
    }

    setRemoteServer(url.getHost(), url.getPort());

    // (String driver, String type, String callback
    var registrationData = new JsonObject()
      .put("name", getImplementation().getName())
      .put("type", "reporter")
      .put("secret", getImplementation().getSecret())
      .put("live", getImplementation().isLiveReporter())
      .put("callback", "http://" + extensionServer.getHostName() + ":" + extensionServer.getPort() + "/");
    context.response().setStatusCode(200);
    context.json(registrationData);
    logger.info("Accepted server side reporter registration");
  }

  private ReporterExtension getImplementation() {
    return implementation;
  }

  private void handleEndReport(RoutingContext context) {
    JsonObject data = context.body().asJsonObject();
    String hostAddress = context.request().remoteAddress().hostAddress();
    String hostName = context.request().remoteAddress().hostName();

    if (data == null) {
      closeInvalidDataRequest(context, hostAddress, hostName);
      return;
    }

    Session session = null;
    try {
      session = jsonToSession(data);
    } catch (IllegalArgumentException e) {
      closeWithMissingSession(context, e, hostAddress, hostName);
      return;
    }

    int rc = implementation.sessionEndLog(session);
    context.response().setStatusCode(rc);
    context.json("ok");
  }

  private static void closeWithMissingSession(RoutingContext context, IllegalArgumentException e, String hostAddress, String hostName) {
    context.response().setStatusCode(400);
    context.json(new JsonObject().put("error", "missing session id"));
    if (e != null)
      LOGGER.warn("Received invalid session data for reporter processing. source=\"{} ({})\" reason=\"{}\"", hostAddress, hostName, e.getMessage());
    else
      LOGGER.warn("Received invalid session data for reporter processing. source=\"{} ({})\"", hostAddress, hostName);
  }

  private static void closeInvalidDataRequest(RoutingContext context, String hostAddress, String hostName) {
    context.response().setStatusCode(400);
    context.json(new JsonObject().put("error", "missing payload"));
    LOGGER.warn("Received invalid request. source=\"{} ({})\" reason=\"missing payload\"", hostAddress, hostName);
  }

  private Session jsonToSession(JsonObject data) throws IllegalArgumentException {
    // Extract UUID
    String uuid = data.getString("uuid");
    if (uuid == null) {
      throw new IllegalArgumentException("Missing or invalid UUID in JSON data");
    }

    // Extract SessionContext
    JsonObject contextJson = data.getJsonObject("context");
    if (contextJson == null) {
      throw new IllegalArgumentException("Missing session context in JSON data");
    }

    List<SessionLogMessage> logMessages = new ArrayList<>();
    JsonArray logArray = contextJson.getJsonArray("log");
    if (logArray != null) {
      for (Object logElement : logArray) {
        if (!(logElement instanceof JsonObject logObject)) {
          throw new IllegalArgumentException("Invalid log entry format");
        }

        String timestampString = logObject.getString("timestamp");
        if (timestampString == null) {
          throw new IllegalArgumentException("Missing timestamp in log entry");
        }
        Date timestamp = Date.from(OffsetDateTime.parse(timestampString).toInstant());

        String type = logObject.getString("type");
        if (type == null) {
          throw new IllegalArgumentException("Missing type in log entry");
        }

        String message = logObject.getString("message");
        if (message == null) {
          throw new IllegalArgumentException("Missing message in log entry");
        }

        logMessages.add(new SessionLogMessage(timestamp, type, message));
      }
    }

    SessionContext context = new SessionContext(logMessages);
    return new Session(uuid, context);
  }

  private void handleLiveReport(RoutingContext context) {
    JsonObject data = context.body().asJsonObject();
    String hostAddress = context.request().remoteAddress().hostAddress();
    String hostName = context.request().remoteAddress().hostName();

    if (data == null) {
      closeInvalidDataRequest(context, hostAddress, hostName);
      return;
    }

    String uuid = data.getString("session");
    if (uuid == null) {
      closeWithMissingSession(context, null, hostAddress, hostName);
    }

    LOGGER.info("Received live logging. session=\"{}\" source=\"{} ({})\"", uuid, hostAddress, hostName);

    JsonObject msg = data.getJsonObject("message");
    if (msg == null) {
      context.response().setStatusCode(400);
      context.json(new JsonObject().put("error", "missing message data"));
      LOGGER.warn("Live logging is missing message data. source=\"{} ({})\"", hostAddress, hostName);
      return;
    }

    String type = msg.getString("type");
    if (type == null) {
      context.response().setStatusCode(400);
      context.json(new JsonObject().put("error", "missing message type"));
      LOGGER.warn("Live logging is missing message typeg. source=\"{} ({})\"", hostAddress, hostName);
      return;
    }

    String content = msg.getString("message");
    if (content == null) {
      context.response().setStatusCode(400);
      context.json(new JsonObject().put("error", "missing message content"));
      LOGGER.warn("Live logging is missing message content. source=\"{} ({})\"", hostAddress, hostName);
      return;
    }


    int rc = implementation.liveLog(uuid, type, content);
    context.response().setStatusCode(rc);
    context.json("ok");
  }

  @Override
  public void registerRemote(Vertx vertx) {
    throw new RuntimeException("not implemented");
  }


  @Override
  public void setRemoteServer(String serverHostName, int serverPort) {
    LogManager.getLogger().info("Updating remote server URL. host={} port={}", serverHostName, serverPort);
  }

  @Override
  public void setExtensionServer(BabylonExtensionServer extensionServer) {
    this.extensionServer = extensionServer;
  }
}
