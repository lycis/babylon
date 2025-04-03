package at.deder.babylon.examples.extensions;

import at.deder.babylon.client.Session;
import at.deder.babylon.client.SessionLogMessage;
import at.deder.babylon.extension.BabylonExtensionServer;
import at.deder.babylon.extension.ReporterExtension;
import org.apache.logging.log4j.LogManager;
import org.apache.logging.log4j.Logger;

import java.io.*;
import java.nio.file.*;
import java.util.*;
import java.util.concurrent.ConcurrentHashMap;

public class LiveDashboardReporter implements ReporterExtension {
    static Logger LOGGER = LogManager.getLogger("LiveDashboardReporter");
    private static final Path DASHBOARD_PATH = Paths.get("logs", "dashboard.html");
    private static final Path LOGS_DIR = Paths.get("logs");
    private static final Map<String, String> sessionStates = new ConcurrentHashMap<>();

    public static void main(String... args) {
        BabylonExtensionServer
                .forReporter(new LiveDashboardReporter())
                .setPort(9095)
                .run();
    }

    @Override
    public int liveLog(String sessionId, String type, String message) {
        sessionStates.put(sessionId, "running");
        updateDashboard();
        appendToSessionLog(sessionId, String.format("[%s] %s: %s\n", new Date(), type, message));
        return 200;
    }

    @Override
    public int sessionEndLog(Session session) {
        LOGGER.info("Processing session end log. session={} messagesCount={}", session.uuid(), session.context().log().size());

        Path sessionLogFile = LOGS_DIR.resolve(session.uuid() + ".html");
        try {
            Files.createDirectories(LOGS_DIR);
            try (BufferedWriter writer = new BufferedWriter(new FileWriter(sessionLogFile.toFile()))) {
                writer.write("<html><head><title>Session Log - " + session.uuid() + "</title>");
                writer.write("<style>body { font-family: Arial, sans-serif; margin: 20px; } pre { background: #f4f4f4; padding: 10px; } h1 { color: #333; }</style>");
                writer.write("</head><body>");
                writer.write("<h1>Session Log: " + session.uuid() + "</h1>");
                writer.write("<pre>");
                for (SessionLogMessage logMessage : session.context().log()) {
                    writer.write(String.format("[%s] %s: %s\n", logMessage.timestamp(), logMessage.type(), logMessage.message()));
                }
                writer.write("</pre></body></html>");
            }
            sessionStates.put(session.uuid(), "ended"); // Mark session as ended
            updateDashboard();
            LOGGER.info("Session log written to file: {}", sessionLogFile);
            return 200;
        } catch (IOException e) {
            LOGGER.error("Failed to write session log to file", e);
            return 500;
        }
    }

    @Override
    public boolean isLiveReporter() {
        return true;
    }

    @Override
    public String getName() {
        return "LiveDashboardReporter";
    }

    @Override
    public String getSecret() {
        return "liveDashboardSecret";
    }

    private void updateDashboard() {
        try (BufferedWriter writer = new BufferedWriter(new FileWriter(DASHBOARD_PATH.toFile()))) {
            writer.write("<html><head><title>Live Dashboard</title>");
            writer.write("<style>body { font-family: Arial, sans-serif; margin: 20px; } table { width: 100%; border-collapse: collapse; } th, td { border: 1px solid #ddd; padding: 8px; text-align: left; } th { background: #f4f4f4; }</style>");
            writer.write("</head><body>");
            writer.write("<h1>Live Sessions Dashboard</h1>");
            writer.write("<table><tr><th>Session ID</th><th>Status</th><th>Log</th></tr>");
            for (Map.Entry<String, String> entry : sessionStates.entrySet()) {
                writer.write(String.format("<tr><td>%s</td><td>%s</td><td><a href='%s.html'>View Log</a></td></tr>",
                        entry.getKey(), entry.getValue(), entry.getKey()));
            }
            writer.write("</table></body></html>");
        } catch (IOException e) {
            LOGGER.error("Failed to update dashboard", e);
        }
    }

    private void appendToSessionLog(String sessionId, String logEntry) {
        Path sessionLogFile = LOGS_DIR.resolve(sessionId + ".html");
        try {
            Files.createDirectories(LOGS_DIR);
            try (BufferedWriter writer = new BufferedWriter(new FileWriter(sessionLogFile.toFile(), true))) {
                writer.write(logEntry);
            }
        } catch (IOException e) {
            LOGGER.error("Failed to write to live session log", e);
        }
    }
}
