# Ethereum RPC Pool

This project is a Go-based proxy server designed to distribute Ethereum JSON-RPC requests across multiple RPC endpoints. It acts as a smart load balancer, enhancing reliability and potentially reducing costs by intelligently routing requests.

## What it Does

The Ethereum RPC Pool serves as a central point for all your Ethereum JSON-RPC requests. Instead of directly connecting to a single RPC provider, you send your requests to this proxy. It then forwards these requests to one of several configured RPC endpoints, distributing the load and providing resilience against single-point failures.

Key functionalities include:

-   **Load Balancing:** Distributes incoming JSON-RPC `POST` requests across a list of configured RPC endpoints.
-   **RPC Health Monitoring:** Continuously monitors the health and performance of each configured RPC endpoint by periodically fetching the latest block number and measuring response times.
-   **Intelligent Routing:** Currently uses a **round-robin** algorithm to select the next available RPC. (Future enhancements could include least-latency routing based on collected performance metrics).
-   **Response Caching:** Caches responses for common and frequently requested methods like `eth_blockNumber` and `eth_getBlockByNumber` to reduce redundant calls to upstream RPCs and improve response times.
-   **Error Handling:** Gracefully handles errors from upstream RPCs and returns them in the standard Ethereum RPC error format.
-   **Configurable:** Easily configured via environment variables for RPC endpoints and server port.
-   **Containerized:** Provided with a `Dockerfile` for easy deployment in containerized environments.

## How it Works

1.  **Initialization:** Upon startup, the server reads a comma-separated list of RPC URLs from the `RPC_LIST` environment variable. It initializes the status for each RPC, marking them as offline initially.
2.  **Background Monitoring:** A background goroutine periodically sends `eth_blockNumber` requests to *all* configured RPCs. For each RPC, it records:
    *   Whether the RPC is `Online` or `Offline`.
    *   The `ResponseTime` (latency) of the `eth_blockNumber` request.
    *   The `BlockNumber` returned by the RPC.
    *   A `Timestamp` of the last successful check.
    This data is stored in an in-memory cache (`rpcStatusCache`).
3.  **Request Handling (`RPCHandler`):**
    *   When a client sends a `POST` request to the proxy, the `RPCHandler` first checks if the request can be served from the internal cache (e.g., `eth_blockNumber` or `eth_getBlockByNumber`).
    *   If the request is cacheable and the data is fresh, the cached response is returned immediately.
    *   If not cacheable or cache is stale, the handler selects an RPC endpoint using the **round-robin** algorithm.
    *   The request is then proxied to the selected upstream RPC.
    *   The response from the upstream RPC is returned to the client.
4.  **Status Endpoint (`/status`):** A dedicated `/status` endpoint provides a JSON overview of the health and performance of all configured RPCs, including their online status, last fetched block number, and response time in milliseconds.

## Getting Started

### Prerequisites

-   Go 1.19 or later
-   Docker (for containerized deployment)

### Environment Variables

-   `RPC_LIST`: A comma-separated list of RPC endpoints (e.g., `http://rpc1.example.com,http://rpc2.example.com`). **Required.**
-   `PORT`: The port on which the server will listen (default is `8080`).
-   `BLOCK_NUMBER_FETCH_INTERVAL_SECONDS`: Interval in seconds for background RPC health checks (default is `10`).

### Installation

1.  Clone the repository:

    ```sh
    git clone https://github.com/itxtoledo/ethereum-rpc-pool.git
    cd ethereum-rpc-pool
    ```

2.  Set the environment variables (you can use a `.env` file or export them directly):

    ```sh
    export RPC_LIST="https://mainnet.infura.io/v3/YOUR_INFURA_PROJECT_ID,https://eth-mainnet.alchemyapi.io/v2/YOUR_ALCHEMY_API_KEY"
    export PORT=8080
    export BLOCK_NUMBER_FETCH_INTERVAL_SECONDS=5
    ```

3.  Run the application:

    ```sh
    go run .
    ```

### Building and Running with Docker

1.  Build the Docker image:

    ```sh
    docker build -t ethereum-rpc-pool .
    ```

2.  Run the Docker container:

    ```sh
    docker run -d -p 8080:8080 -e RPC_LIST="https://mainnet.infura.io/v3/YOUR_INFURA_PROJECT_ID,https://eth-mainnet.alchemyapi.io/v2/YOUR_ALCHEMY_API_KEY" -e PORT=8080 ethereum-rpc-pool
    ```

## One-Click Deployment

### Railway

1.  Click the "Deploy on Railway" button above
2.  Set the required environment variable `RPC_LIST` with your comma-separated RPC endpoints
3.  Deploy and your Ethereum RPC Pool will be live

## Usage

Once the server is running, you can send standard Ethereum JSON-RPC `POST` requests to its address.

Example POST request (fetching the latest block number):

```sh
curl -X POST http://localhost:8080/ -H "Content-Type: application/json" -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}'
```

### Status Endpoint

You can check the real-time status and performance metrics of each configured RPC by sending a `GET` request to the `/status` endpoint.

```sh
curl http://localhost:8080/status
```

This will return a JSON object with the status of each RPC, including its online status, the latest block number it reported, and its response time in milliseconds.

## Project Structure

The project is organized into several key directories and files:

-   `main.go`: The application's entry point. It handles environment variable loading, initializes RPC status, sets up HTTP routes (`/` for RPC proxying and `/status` for health checks), and starts the HTTP server.
-   `handlers/`: Contains the HTTP handler functions for the RPC proxy and status endpoint.
    -   `rpc.go`: Implements the core logic for proxying JSON-RPC requests, including cache handling for `eth_blockNumber` and `eth_getBlockByNumber`, and forwarding requests to upstream RPCs.
    -   `cache.go`: Manages the in-memory cache for RPC status (online/offline, response time, block number) and block data. It provides functions to set and retrieve RPC and block cache information.
    -   `error.go`: Provides utility functions for sending standardized JSON-RPC error responses.
-   `utils/`: Contains utility functions used across the project.
    -   `round_robin.go`: Implements the round-robin algorithm for selecting the next RPC endpoint from the configured list.
-   `.github/workflows/on_pr.yaml`: GitHub Actions workflow configuration for Continuous Integration (CI), running tests and linting on Pull Requests.
-   `Dockerfile`: Defines the multi-stage Docker build process for creating a lightweight container image of the application.
-   `.env.example`: An example file demonstrating the required environment variables.
-   `go.mod` & `go.sum`: Go module definition and dependency checksums.
-   `LICENSE`: The project's license (MIT License).

## Contributing

Contributions are welcome! Please feel free to open issues or submit pull requests.

1.  Fork the repository.
2.  Create a new branch (`git checkout -b feature-branch`).
3.  Commit your changes (`git commit -am 'Add some feature'`).
4.  Push to the branch (`git push origin feature-branch`).
5.  Create a new Pull Request.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.