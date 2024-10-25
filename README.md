
# Membership Algorithm

## Overview

This project implements a membership algorithm in a distributed system environment. The membership service allows peers (servers) to join and leave the network, detect failures, and handle leader failures, ensuring that all peers maintain a consistent view of the current members. This implementation is done in Go, simulating the interactions between peers using a combination of TCP and UDP communications for reliability and broadcasting. Please refer to REPORT.md for details on the algorithm.

The project utilizes a Makefile to manage building, running, and cleaning up Docker containers implemented using Docker and Docker Compose. The Makefile simplifies the process of handling multiple Docker Compose files and allows for automated testing of different scenarios.

### 1. Build the Docker Image
To build the Docker image used for the membership algorithm:
```sh
make build
```
This command will build a Docker image named `prj3`.

### 2. Run a Specific Test Case
To run a specific test case using one of the Docker Compose files:
```sh
make run TEST_CASE=<number>
```
Replace `<number>` with the value from 1 to 4, corresponding to one of the following Docker Compose files:
- **1**: Runs using `docker-compose-testcase-1.yml`
- **2**: Runs using `docker-compose-testcase-2.yml`
- **3**: Runs using `docker-compose-testcase-3.yml`
- **4**: Runs using `docker-compose-testcase-4.yml`

### 3. Stop the Running Containers
To stop a specific test case that is running:
```sh
make stop TEST_CASE=<number>
```
Replace `<number>` with the value from 1 to 4 to stop the corresponding Docker Compose setup.

### 4. Clean Up Docker Containers, Images, and Volumes
To clean up all resources, including stopping containers, removing containers, volumes, and images:
```sh
make clean
```
This command stops the services, removes the containers, associated volumes, and deletes the Docker image.

### 5. Rebuild and Run a Specific Test Case
To completely rebuild the Docker image and run a specific test case:
```sh
make rebuild TEST_CASE=<number>
```
This command cleans up all resources, rebuilds the Docker image, and then runs the specified test case as described in the `run` target.

## Using Docker and Docker Compose Directly
Using Docker and Docker Compose commands directly:

### Build the Docker Image
To build the Docker image:
```sh
docker build -t prj3 .
```

### Run Containers for Each Test Case
To run containers for each test case, use the following commands:
```sh
docker compose -f docker-compose-testcase-1.yml up
docker compose -f docker-compose-testcase-2.yml up
docker compose -f docker-compose-testcase-3.yml up
docker compose -f docker-compose-testcase-4.yml up
```

### Stop and Remove Containers
To stop and remove the containers after running, use the following commands:
```sh
docker compose -f docker-compose-testcase-1.yml down
docker compose -f docker-compose-testcase-2.yml down
docker compose -f docker-compose-testcase-3.yml down
docker compose -f docker-compose-testcase-4.yml down
```

## Test Case Descriptions
Each Docker Compose file corresponds to a specific test case for the membership algorithm. Here is a brief description of what each test case does:

1. **Test Case 1**: This test case simulates a scenario where peers join the network sequentially. It starts with only the leader, and new peers join one by one, updating the membership view accordingly.

2. **Test Case 2**: This test case introduces a scenario where a peer crashes, and the remaining peers must detect the failure and update the membership accordingly.

3. **Test Case 3**: This test case simulates peers leaving the network sequentially, starting with all peers active and having each peer leave one by one until only the leader remains.

4. **Test Case 4**: This test case simulates a leader failure during an operation. The peer with the lowest ID will take over as the new leader and ensure that pending operations are completed.

## Important Notes
- The Makefile assumes you have Docker and Docker Compose installed on your machine.
- Each Docker Compose file (`docker-compose-testcase-1.yml`, `docker-compose-testcase-2.yml`, `docker-compose-testcase-3.yml`, `docker-compose-testcase-4.yml`) represents a different configuration or test scenario for the membership algorithm.
- You can specify which test case to run or stop by setting the `TEST_CASE` variable.

## Example Usage
To build, run, and clean up the project, you could execute the following commands in sequence:
```sh
make build
make run TEST_CASE=3
make stop TEST_CASE=3
make clean
```
This would first build the Docker image, run the project using the third Docker Compose file (`docker-compose-testcase-3.yml`), stop the services, and finally clean up all the resources used.

## Implementation Specifc

### Leader Failure Simulation (Custom Test Case 4)

In the custom test case 4, the leader initiates a deletion of a peer but crashes before completion. The new leader finishes this pending operation to ensure consistency. During this process, the leader is sending a REQ to delete peer five here (The last member). And since this is simulation, the peer five is not actually down, and will no longer participate in the membership. The new Leader elected, completes this operation even though it would not have received this request by talking to other members. And due to this, the peer five would miss heartbeats from everyone in the membership and think they are unreachable, which is expected for this simulation.