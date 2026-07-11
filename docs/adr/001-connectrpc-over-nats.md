# ADR 001: ConnectRPC over NATS Request-Reply for control plane configuration

*Status: Accepted | Author: saphalpdyl*

## Context
Previously, the communication between the agent and the control plane ( cepheus-server ) was done over a REST API. As per the project's philosophy, communication between services must be implementation agnostic. Thus, we require a platform-agnostic schema shared by the two services to communicate back-and-forth. 

## Decision

Communication through the already existing NATS connection would be a viable solution that would look operationally simpler on paper. However, given the constraints and deployment model, we have decided that NATS shall only have one job: to transport telemetry to the processors. Meanwhile, the connection with the server is maintained by a seperate ConnectRPC connection.

## Positive Consequences
- Rigid type safety
- Operationally simpler to implement native HTTP middleware since ConnectRPC maps cleanly
- Separation of Concerns

## Negative Consequences
- Dual-client operational overhead