package at.deder.babylon.client;

import java.util.List;

public record SessionContext(List<SessionLogMessage> log) {
}
