package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/skaegi/legion-router/pkg/config"
	"github.com/skaegi/legion-router/pkg/filter"
)

func main() {
	configPath := flag.String("config", "/etc/legion-router/config.yaml", "Path to configuration file")
	flag.Parse()

	// Enable IP forwarding
	if err := enableIPForwarding(); err != nil {
		log.Printf("Warning: failed to enable IP forwarding: %v", err)
		log.Printf("You may need to run: echo 1 > /proc/sys/net/ipv4/ip_forward")
	}

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Printf("Loaded configuration version %s with %d rules", cfg.Version, len(cfg.Rules))

	// Create and start the filter
	f, err := filter.New(cfg, *configPath)
	if err != nil {
		log.Fatalf("Failed to create filter: %v", err)
	}

	if err := f.Start(); err != nil {
		log.Fatalf("Failed to start filter: %v", err)
	}

	log.Println("Legion Router started successfully")

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")
	if err := f.Stop(); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}
}

// enableIPForwarding enables IP forwarding on Linux
func enableIPForwarding() error {
	return os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1\n"), 0644)
}
