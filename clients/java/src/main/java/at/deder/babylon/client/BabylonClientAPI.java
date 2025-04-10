package at.deder.babylon.client;

import feign.Headers;
import feign.Param;
import feign.RequestLine;

public interface BabylonClientAPI {
    @RequestLine("GET /session")
    Session createNewSession();

    @RequestLine("GET /session/{id}")
    Session sessionInfo(@Param("id") String id);

    @RequestLine("POST /actor/execute")
    @Headers("Content-Type: application/json")
    ExecutionResult executeActorAction(ActorAction action);

    @RequestLine("DELETE /session/{id}")
    void endSession(@Param("id") String id);

    @RequestLine("POST /driver/execute")
    @Headers("Content-Type: application/json")
    ExecutionResult executeDriverAction(DriverAction actorAction);
}
