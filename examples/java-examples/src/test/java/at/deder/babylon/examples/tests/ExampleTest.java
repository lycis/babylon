package at.deder.babylon.examples.tests;

import at.deder.babylon.client.BabylonClient;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import static org.assertj.core.api.Assertions.assertThat;

public class ExampleTest {
    private static BabylonClient babylon;

    @BeforeAll
    public static void setupBabylon() {
        babylon = BabylonClient.createFor("http://localhost:9090/");
    }

    @BeforeEach
    public void beforeEach() {
        babylon.session();
    }

    @AfterEach
    public void afterEach() {
        babylon.endSession();
    }

    @Test
    public void simpleDriverAction() {
        var result = babylon.driver("example").action("testAction").execute();
        assertThat(result.success()).isTrue();

    }

    @Test
    public void actionWithParameters() {
        var result = babylon.driver("example")
                .action("actionWithParameters")
                .parameter("foo", "bar")
                .execute();
        assertThat(result.success()).isTrue();
        assertThat(result.message()).isEqualTo("action executed with parameters: {foo=bar}");

    }
}
