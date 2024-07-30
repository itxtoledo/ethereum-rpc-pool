package main

import (
	"ethereum-rpc-pool/handlers"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

func main() {
	// Path to the .env file
	envFile := filepath.Join(".", ".env")

	// Check if the .env file exists
	if _, err := os.Stat(envFile); err == nil {
		log.Println("Loading environment variables from .env file")
		// Attempt to load environment variables from the .env file
		if err := godotenv.Load(envFile); err != nil {
			log.Printf("Warning: Could not load .env file: %v. Proceeding with system environment variables.", err)
		}
	} else if os.IsNotExist(err) {
		log.Println("No .env file found. Proceeding with system environment variables.")
	} else if !os.IsNotExist(err) {
		log.Printf("Warning: Could not check for .env file existence: %v. Proceeding with system environment variables.", err)
	}

	// Load RPCs from the environment variable
	rpcList := os.Getenv("RPC_LIST")
	if rpcList == "" {
		log.Fatal("The environment variable RPC_LIST is not defined")
	}
	handlers.SetRPCs(rpcList)

	// Load the port from the environment variable, default to 8080 if not set
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Set up the handlers
	http.HandleFunc("/", handlers.RPCHandler)

	// Start the server on the specified port
	log.Printf("Server started on port %s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
