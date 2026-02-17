// Test program to verify janitor agent works with Emergent server
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/emergent-company/specmcp/internal/emergent"
	"github.com/emergent-company/specmcp/internal/tools/janitor"
)

func main() {
	// Set up logger
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Get config from environment
	serverURL := os.Getenv("EMERGENT_URL")
	if serverURL == "" {
		serverURL = "http://localhost:3002"
	}
	token := os.Getenv("EMERGENT_TOKEN")
	if token == "" {
		fmt.Fprintf(os.Stderr, "EMERGENT_TOKEN required\n")
		os.Exit(1)
	}

	projectID := os.Getenv("EMERGENT_PROJECT_ID")
	if projectID == "" {
		fmt.Fprintf(os.Stderr, "EMERGENT_PROJECT_ID required\n")
		os.Exit(1)
	}

	logger.Info("connecting to Emergent", "url", serverURL, "project_id", projectID)

	// Create client factory with token as admin token, 5 retries, 5 min long outage interval, 20 failure threshold
	factory := emergent.NewClientFactory(serverURL, token, 5, 5, 20, logger)

	// Create context with token and project
	ctx := emergent.WithToken(context.Background(), token)
	client, err := factory.ClientFor(ctx)
	if err != nil {
		logger.Error("failed to create client", "error", err)
		os.Exit(1)
	}

	// Create a test Change to verify we can write
	logger.Info("creating test Change")
	key := "janitor-hello-world"
	changeObj, err := client.CreateObject(ctx, emergent.TypeChange, &key, map[string]any{
		"status":      "draft",
		"description": "Hello World test for janitor agent",
		"created_at":  "2026-02-16T18:00:00Z",
	}, nil)
	if err != nil {
		logger.Error("failed to create test Change", "error", err)
		os.Exit(1)
	}
	logger.Info("created test Change", "id", changeObj.ID, "key", key)

	// Create a Spec with bad naming (not kebab-case)
	badKey := "Bad_Naming_Convention"
	specObj, err := client.CreateObject(ctx, emergent.TypeSpec, &badKey, map[string]any{
		"status":      "draft",
		"description": "Spec with bad naming to test janitor",
	}, nil)
	if err != nil {
		logger.Error("failed to create test Spec", "error", err)
		os.Exit(1)
	}
	logger.Info("created test Spec with bad naming", "id", specObj.ID, "key", badKey)

	// Link the spec to the change
	_, err = client.CreateRelationship(ctx, emergent.RelHasSpec, changeObj.ID, specObj.ID, nil)
	if err != nil {
		logger.Error("failed to link spec to change", "error", err)
	} else {
		logger.Info("linked spec to change")
	}

	// Now run the janitor
	logger.Info("running janitor agent")

	janitorTool := janitor.NewJanitorRun(factory, logger)

	params := map[string]any{
		"scope":           "all",
		"create_proposal": false,
		"auto_fix":        false,
	}

	paramsJSON, _ := json.Marshal(params)
	result, err := janitorTool.Execute(ctx, paramsJSON)
	if err != nil {
		logger.Error("janitor execution failed", "error", err)
		os.Exit(1)
	}

	if result.IsError {
		logger.Error("janitor returned error", "content", result.Content)
		os.Exit(1)
	}

	// Pretty print the result
	fmt.Println("\n=== JANITOR REPORT ===")
	for _, content := range result.Content {
		if content.Type == "text" {
			// Try to parse as JSON and pretty print
			var report map[string]any
			if err := json.Unmarshal([]byte(content.Text), &report); err == nil {
				prettyJSON, _ := json.MarshalIndent(report, "", "  ")
				fmt.Println(string(prettyJSON))
			} else {
				fmt.Println(content.Text)
			}
		}
	}

	logger.Info("janitor test complete!")
}
