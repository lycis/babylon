package at.deder.babylon.extension;

import at.deder.babylon.client.Session;

public interface ReporterExtension {
  /**
   * Processes a live log message, if live logging was enabled for this reporter.
   *
   * @param sessionId server-side UUID of the session
   * @param type type of the received message
   * @param message message content
   * @return HTTP Status Code (200 for OK)
   */
  int liveLog(String sessionId, String type, String message);

  /**
   * Create a full session log. Only will be called if live logging is not enabled for the reporter.
   * @param session session info and context
   * @return HTTP Status Code (200 for OK)
   */
  int sessionEndLog(Session session);

  /**
   * Indicates whether this is a live reporter or end-of-session reporter
   * @return return true if logs will be processed live
   */
  boolean isLiveReporter();

  String getName();

  String getSecret();
}
