package at.deder.babylon.examples.extensions;


import at.deder.babylon.client.Session;
import at.deder.babylon.client.SessionLogMessage;
import at.deder.babylon.extension.BabylonExtensionServer;
import at.deder.babylon.extension.ReporterExtension;
import org.apache.logging.log4j.LogManager;
import org.apache.logging.log4j.Logger;

import java.io.BufferedWriter;
import java.io.FileWriter;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;

public class ExampleFileLogReporter implements ReporterExtension {
    static Logger LOGGER = LogManager.getLogger("FileLogReporter");

    public static void main(String... args) {
        BabylonExtensionServer
                .forReporter(new ExampleFileLogReporter())
                .setPort(9094)
                .run();
    }


    @Override
    public int liveLog(String sessionId, String type, String message) {
        return 0;
    }

    @Override
    public int sessionEndLog(Session session) {
        LOGGER.info("Processing session end log. session={} messagesCount={}", session.uuid(), session.context().log().size());

        Path logFilePath = Paths.get("logs", session.uuid() + ".txt");
        try {
            Files.createDirectories(logFilePath.getParent());
            try (BufferedWriter writer = new BufferedWriter(new FileWriter(logFilePath.toFile()))) {
                for (SessionLogMessage logMessage : session.context().log()) {
                    writer.write(String.format("[%s] %s: %s\n", logMessage.timestamp(), logMessage.type(), logMessage.message()));
                }
            }
            LOGGER.info("Session log written to file: {}", logFilePath);
            return 200;
        } catch (IOException e) {
            LOGGER.error("Failed to write session log to file", e);
            return 500;
        }
    }

    @Override
    public boolean isLiveReporter() {
        return false;
    }

    @Override
    public String getName() {
        return "FileReporter";
    }

    @Override
    public String getSecret() {
        return "fileReporterSecret";
    }
}