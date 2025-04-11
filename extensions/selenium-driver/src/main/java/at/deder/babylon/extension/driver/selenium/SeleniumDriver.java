package at.deder.babylon.extension.driver.selenium;

import at.deder.babylon.client.BabylonClient;
import at.deder.babylon.extension.BabylonExtensionServer;
import at.deder.babylon.extension.ExecutableExtension;
import at.deder.babylon.extension.ExecutionResult;

import org.apache.logging.log4j.LogManager;
import org.apache.logging.log4j.Logger;
import org.openqa.selenium.By;
import org.openqa.selenium.WebDriver;
import org.openqa.selenium.WebElement;
import org.openqa.selenium.chrome.ChromeDriver;

import java.util.HashMap;
import java.util.Map;
import java.util.concurrent.TimeUnit;

public class SeleniumDriver implements ExecutableExtension {
    private static Logger LOGGER = LogManager.getLogger();
    private Map<String, WebDriver> sessions = new HashMap<>();

    public static void main(String... args) {
        BabylonExtensionServer
                .forDriver(new SeleniumDriver())
                .setPort(9089)
                .run();
    }

    private WebDriver ensureSession(String uid) {
        if (!sessions.containsKey(uid)) {
            LOGGER.info("No current session. Creating new session.");
            // You can swap in FirefoxDriver, EdgeDriver, etc.
            var driver = new ChromeDriver();
            driver.manage().timeouts().implicitlyWait(10, TimeUnit.SECONDS);
            sessions.put(uid, driver);
        }
        return sessions.get(uid);
    }

    @Override
    public ExecutionResult execute(String action, Map<String, Object> parameters, BabylonClient api) {
        LOGGER.info("Received action. action={}", action);
        var driver = ensureSession(api.session().uuid());
        if(driver == null) {
            LOGGER.error("Missing session driver. session={}", api.session().uuid());
        }
        try {
            switch (action.toUpperCase()) {
                case "NAVIGATE" -> {
                    String url = (String) parameters.get("url");
                    driver.get(url);
                    return ExecutionResult.success("Navigated to " + url);
                }
                case "CLICK" -> {
                    WebElement element = findElement(parameters, driver);
                    element.click();
                    return ExecutionResult.success("Clicked element");
                }
                case "TYPE" -> {
                    WebElement element = findElement(parameters, driver);
                    String text = (String) parameters.get("text");
                    element.clear();
                    element.sendKeys(text);
                    return ExecutionResult.success("Typed text");
                }
                case "SUBMIT" -> {
                    WebElement element = findElement(parameters, driver);
                    element.submit();
                    return ExecutionResult.success("Form submitted");
                }
                case "CLOSE" -> {
                    if (driver != null) {
                        driver.quit();
                        driver = null;
                        return ExecutionResult.success("Session closed");
                    } else {
                        return ExecutionResult.failure("No session to close");
                    }
                }
                default -> {
                    return ExecutionResult.failure("Unknown action: " + action);
                }
            }
        } catch (Exception e) {
            return ExecutionResult.failure("Selenium action failed: " + e.getMessage());
        }
    }

    private WebElement findElement(Map<String, Object> parameters, WebDriver driver) {
        String by = (String) parameters.getOrDefault("by", "css"); // default to CSS
        String selector = (String) parameters.get("selector");

        return switch (by.toLowerCase()) {
            case "id" -> driver.findElement(By.id(selector));
            case "name" -> driver.findElement(By.name(selector));
            case "xpath" -> driver.findElement(By.xpath(selector));
            case "css" -> driver.findElement(By.cssSelector(selector));
            case "tag" -> driver.findElement(By.tagName(selector));
            case "linktext" -> driver.findElement(By.linkText(selector));
            default -> throw new IllegalArgumentException("Unknown selector type: " + by);
        };
    }

    @Override
    public String getName() {
        return "selenium";
    }

    @Override
    public String getType() {
        return "selenium";
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
        return false; // Connect only when actions are invoked
    }

    @Override
    public void onSessionEnd(String id) {
        LOGGER.info("Session has ended. id={}", id);
        if(sessions.containsKey(id)) {
            LOGGER.info("Closing driver for session on end. id={}", id);
            sessions.get(id).quit();
            sessions.remove(id);
        }
    }
}
