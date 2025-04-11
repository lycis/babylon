package at.deder.babylon.extension;

import io.vertx.core.json.JsonObject;

import java.io.Serializable;
import java.util.Map;

public record ExecutionResult(boolean success, String message, Object data) {

  public ExecutionResult(boolean success, String message) {
    this(success, message, null);
  }

  public static ExecutionResult failure(String message) {
    return new ExecutionResult(false, message);
  }

  public static ExecutionResult success(String message) {
    return new ExecutionResult(true, message);
  }

  public JsonObject toJson() {
    return new JsonObject()
      .put("success", success)
      .put("message", message);
  }
}
