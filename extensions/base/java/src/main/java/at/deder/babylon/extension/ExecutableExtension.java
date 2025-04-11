package at.deder.babylon.extension;

import at.deder.babylon.client.BabylonClient;

import java.util.Map;

public interface ExecutableExtension {
  /**
   * Execute the given action with parameters.
   *
   * @param action     The action to execute.
   * @param parameters Parameters for the action.
   * @return An ExecutionResult containing the outcome.
   */
  ExecutionResult execute(String action, Map<String, Object> parameters, BabylonClient api);

  /**
   * Returns the unique name of this extension.
   */
  String getName();

  /**
   * Returns the type of extension (e.g. "EBanking", "Logistics Application", etc.).
   */
  String getType();

  /**
   * Returns a shared secret between the extension and server that may be to connect depending on the server config.
   */
  String getSecret();

  boolean connectOnStartupEnabled();

  void onSessionEnd(String id);
}
