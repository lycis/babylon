package at.deder.babylon.client;

import at.deder.babylon.extension.driver.ExecutionResult;
import feign.Headers;
import feign.Param;
import feign.RequestLine;

public interface BabylonClientAPI {
    @RequestLine("GET /session")
    Session createNewSession();

    @RequestLine("GET /session/{id}")
    Session sessionInfo(@Param("id") String id);

    @RequestLine("POST /driver/execute")
    @Headers("Content-Type: application/json")
    ExecutionResult action(DriverAction action);

    @RequestLine("DELETE /session/{id}")
    void endSession(@Param("id") String id);
}
