package main

import (
	"flag"
	"fmt"
	"github.com/1Panel-dev/mcp-1panel/operations/app"
	"github.com/1Panel-dev/mcp-1panel/operations/database"
	"github.com/1Panel-dev/mcp-1panel/operations/ssl"
	"github.com/1Panel-dev/mcp-1panel/operations/system"
	"github.com/1Panel-dev/mcp-1panel/operations/website"
	"github.com/1Panel-dev/mcp-1panel/utils"
	"log"
	"os"
	"path/filepath"

	"github.com/mark3labs/mcp-go/server"
)

var (
	Version = utils.Version
)

func setupLogger() (*os.File, error) {
	logDir := "logs"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		fmt.Printf("create log dir error: %v\n", err)
		return nil, err
	}

	logFilePath := filepath.Join(logDir, "mcp-1panel.log")
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		fmt.Printf("open log file error: %v\n", err)
		return nil, err
	}

	log.SetOutput(logFile)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	return logFile, nil
}

func newMCPServer() *server.MCPServer {
	return server.NewMCPServer(
		"github.com/1Panel-dev/mcp-1panel",
		Version,
		server.WithToolCapabilities(true),
		server.WithLogging(),
	)
}

func addTools(s *server.MCPServer) {
	s.AddTool(system.GetSystemInfoTool, system.GetSystemInfoHandle)
	s.AddTool(system.GetDashboardInfoTool, system.GetDashboardInfoHandle)
	s.AddTool(website.ListWebsitesTool, website.ListWebsiteHandle)
	s.AddTool(website.CreateWebsiteTool, website.CreateWebsiteHandle)
	s.AddTool(ssl.ListSSLsTool, ssl.ListSSLHandle)
	s.AddTool(app.InstallMySQLTool, app.InstallMySQLHandle)
	s.AddTool(app.InstallOpenRestyTool, app.InstallOpenRestyHandle)
	s.AddTool(app.ListInstalledAppsTool, app.ListInstalledAppsHandle)
	s.AddTool(ssl.CreateSSLTool, ssl.CreateSSLHandle)
	s.AddTool(database.ListDatabasesTool, database.ListDatabasesHandle)
	s.AddTool(database.CreateDatabaseTool, database.CreateDatabaseHandle)
}

func runServer(transport string, addr string) error {
	mcpServer := newMCPServer()
	addTools(mcpServer)

	if transport == "sse" {
		port, err := utils.GetPortFromAddr(addr)
		if err != nil {
			return err
		}
		log.Printf("SSE server listening on :%s", port)
		sseServer := server.NewSSEServer(mcpServer, server.WithBaseURL(addr))
		if err := sseServer.Start(fmt.Sprintf(":%s", port)); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	} else {
		log.Printf("Run Stdio server")
		if err := server.ServeStdio(mcpServer); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}
	return nil
}

func main() {
	var (
		transport   string
		accessToken string
		host        string
		addr        string
	)
	flag.StringVar(&transport, "transport", "sse", "Transport type (stdio or sse)")
	flag.StringVar(&addr, "addr", "http://localhost:8000", "The base URL for mcp Server")
	flag.StringVar(&accessToken, "token", "", "1Panel api key")
	flag.StringVar(&host, "host", "", "1Panel host (example:http://127.0.0.1:9999)")
	flag.Parse()

	logFile, err := setupLogger()
	if err != nil {
		fmt.Printf("setup logger error: %v\n", err)
		os.Exit(1)
	}
	defer logFile.Close()

	if accessToken != "" {
		utils.SetAccessToken(accessToken)
	}
	if host != "" {
		utils.SetHost(host)
	}

	if err := runServer(transport, addr); err != nil {
		fmt.Printf("server run error: %v\n", err)
		panic(err)
	}
}
