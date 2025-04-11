package at.deder.babylon.extension.restdriver;

import at.deder.babylon.client.BabylonClient;
import at.deder.babylon.extension.BabylonExtensionServer;
import at.deder.babylon.extension.ExecutableExtension;
import at.deder.babylon.extension.ExecutionResult;

import java.io.BufferedReader;
import java.io.InputStreamReader;
import java.io.OutputStream;
import java.net.HttpURLConnection;
import java.net.URI;
import java.net.URL;

import java.nio.charset.StandardCharsets;
import java.util.Map;
import java.util.stream.Collectors;

public class RestApiActor implements ExecutableExtension {

    public static void main(String... args) {
        BabylonExtensionServer
                .forDriver(new RestApiActor())
                .setPort(9088)
                .run();
    }

    @Override
    public ExecutionResult execute(String action, Map<String, Object> parameters, BabylonClient api) {
        try {
            // Expected format: "METHOD:/api/endpoint"
            String[] parts = action.split(":", 2);
            if (parts.length != 2) {
                return ExecutionResult.failure("Invalid action format. Use METHOD:/path");
            }

            String method = parts[0].toUpperCase();
            String path = parts[1];

            String baseUrl = (String) parameters.getOrDefault("baseUrl", "http://localhost:8080");
            URL url = new URI(baseUrl + path).toURL();
            HttpURLConnection conn = (HttpURLConnection) url.openConnection();
            conn.setRequestMethod(method);

            // Set headers if present
            Object headersObj = parameters.get("headers");
            if (headersObj instanceof Map<?, ?> headers) {
                for (Map.Entry<?, ?> entry : headers.entrySet()) {
                    if (entry.getKey() != null && entry.getValue() != null) {
                        conn.setRequestProperty(entry.getKey().toString(), entry.getValue().toString());
                    }
                }
            }

            // Handle body for methods that support output
            boolean hasOutput = method.equals("POST") || method.equals("PUT") || method.equals("PATCH") || method.equals("DELETE");
            if (hasOutput && parameters.containsKey("body")) {
                conn.setDoOutput(true);
                String body = parameters.get("body").toString();
                byte[] input = body.getBytes(StandardCharsets.UTF_8);
                conn.setRequestProperty("Content-Length", Integer.toString(input.length));
                try (OutputStream os = conn.getOutputStream()) {
                    os.write(input);
                }
            }

            int status = conn.getResponseCode();
            BufferedReader reader = new BufferedReader(new InputStreamReader(
                    status >= 200 && status < 300 ? conn.getInputStream() : conn.getErrorStream(),
                    StandardCharsets.UTF_8
            ));
            String response = reader.lines().collect(Collectors.joining("\n"));

            return ExecutionResult.success("status="+status+" response="+response);
        } catch (Exception e) {
            return ExecutionResult.failure("Exception during REST call: " + e.getMessage());
        }
    }

    @Override
    public String getName() {
        return "RestApi";
    }

    @Override
    public String getType() {
        return "REST";
    }

    @Override
    public String getSecret() {
        // 1. Check direct secret in environment variable
        String envSecret = System.getenv("BABYLON_SECRET");
        if (envSecret != null && !envSecret.isBlank()) {
            return envSecret.trim();
        }

        // 2. Check for file path in env or system property
        String secretFilePath = System.getenv("BABYLON_SECRET_FILE");
        if (secretFilePath == null || secretFilePath.isBlank()) {
            secretFilePath = System.getProperty("babylon.secret.file", ".secret"); // default fallback
        }

        try {
            return java.nio.file.Files.readString(java.nio.file.Path.of(secretFilePath)).trim();
        } catch (Exception e) {
            System.err.println("WARNING: Could not read secret file at " + secretFilePath + ": " + e.getMessage());
            // 3. Fallback
            return "default-secret";
        }
    }


    @Override
    public boolean connectOnStartupEnabled() {
        return false;
    }
}
