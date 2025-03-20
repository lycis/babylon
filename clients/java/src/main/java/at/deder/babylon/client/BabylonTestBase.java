package at.deder.babylon.client;

import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.BeforeEach;

public class BabylonTestBase {
    protected Session session;
    protected static BabylonClient babylon;
    private static String babylonServerUrl;

    public BabylonTestBase() {
        setBabylonServerUrl("http://localhost:8080/");
    }

    @BeforeAll
    public static void setupSession() {
        babylon = BabylonClient.createFor(babylonServerUrl);
    }

    @BeforeEach
    public void initTestSession() {
        session = babylon.session();
    }

    @AfterEach
    public void closeSession() {
        babylon.endSession();
    }

    public void setBabylonServerUrl(String babylonServerUrl) {
        BabylonTestBase.babylonServerUrl = babylonServerUrl;
    }
}
