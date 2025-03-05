# Ethereum RPC Pool

This project is a Go-based proxy server designed to distribute POST requests across multiple RPC endpoints using a round-robin algorithm. By using this proxy, you can avoid relying on a single RPC endpoint. Instead, you can configure a list of public RPC endpoints, and the service will handle the distribution of requests among them. This approach can help you avoid the costs associated with paid nodes while ensuring better reliability and load balancing.

[![Deploy on Railway](https://railway.com/button.svg)](https://railway.com/template/CObZnk?referralCode=PgYfrf)

## Features

- Load balancing using round-robin algorithm.
- Handles POST requests and proxies them to one of the configured RPC endpoints.
- Returns responses in the Ethereum RPC format.
- Configurable through environment variables.
- Lightweight Docker container for deployment.

## Getting Started

### Prerequisites

- Go 1.19 or later
- Docker

### Environment Variables

- `RPC_LIST`: A comma-separated list of RPC endpoints.
- `PORT`: The port on which the server will listen (default is 8080).

### Installation

1. Clone the repository:

   ```sh
   git clone https://github.com/yourusername/go-rpc-proxy.git
   cd go-rpc-proxy
   ```

2. Set the environment variables:

   ```sh
   export RPC_LIST=http://rpc1.example.com,http://rpc2.example.com,http://rpc3.example.com
   export PORT=8080
   ```

3. Run the application:
   ```sh
   go run main.go
   ```

### Building and Running with Docker

1. Build the Docker image:

   ```sh
   docker build -t go-rpc-proxy .
   ```

2. Run the Docker container:
   ```sh
   docker run -d -p 8080:8080 -e RPC_LIST="http://rpc1.example.com,http://rpc2.example.com,http://rpc3.example.com" -e PORT=8080 go-rpc-proxy
   ```

## One-Click Deployment

### Railway

1. Click the "Deploy on Railway" button above
2. Set the required environment variable `RPC_LIST` with your comma-separated RPC endpoints
3. Deploy and your Ethereum RPC Pool will be live

### Usage

The server will start and listen on the specified port. You can send POST requests to the server, and it will proxy the requests to one of the configured RPC endpoints.

Example POST request:

```sh
curl -X POST http://localhost:8080/ -H "Content-Type: application/json" -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}'
```

## Code Overview

### `main.go`

The main Go application file:

- Loads the RPC list and port from environment variables.
- Sets up an HTTP server to handle POST requests.
- Uses a round-robin algorithm to select the next RPC endpoint.
- Proxies the request to the selected RPC and returns the response to the client.
- Handles errors and returns them in the Ethereum RPC format.

### `Dockerfile`

A multi-stage Dockerfile:

- **Builder Stage:** Uses the official Golang image to build the Go application.
- **Final Stage:** Uses a minimal Alpine image to run the built application.

## Contributing

1. Fork the repository.
2. Create a new branch (`git checkout -b feature-branch`).
3. Commit your changes (`git commit -am 'Add some feature'`).
4. Push to the branch (`git push origin feature-branch`).
5. Create a new Pull Request.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
