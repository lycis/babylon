package at.deder.babylon.extension;

import io.vertx.core.json.JsonObject;

public record ExecutionResult(boolean success, String message, Object data) {

  public ExecutionResult(boolean success, String message) {
    this(success, message, null);
  }

  public JsonObject toJson() {
    return new JsonObject()
      .put("success", success)
      .put("message", message);
  }
}
