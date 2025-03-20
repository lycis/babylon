package at.deder.babylon.client;

import java.util.Date;

public record SessionLogMessage(Date timestamp, String type, String message) {
}
