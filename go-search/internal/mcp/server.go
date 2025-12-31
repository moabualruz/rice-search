package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sync"
)

type Server struct {
	socketPath string
	handler    *Handler
	listener   net.Listener

	// Active connections
	connsMu sync.RWMutex
	conns   map[net.Conn]struct{}

	log *slog.Logger
}

type ServerConfig struct {
	SocketPath string
	Handler    *Handler
}

func NewServer(cfg ServerConfig) *Server {
	if cfg.SocketPath == "" {
		home, _ := os.UserHomeDir()
		// Use a platform-appropriate directory
		cfg.SocketPath = filepath.Join(home, ".local", "run", "rice-search", "mcp.sock")
	}

	return &Server{
		socketPath: cfg.SocketPath,
		handler:    cfg.Handler,
		conns:      make(map[net.Conn]struct{}),
		log:        slog.Default().With("component", "mcp"),
	}
}

func (s *Server) Start(ctx context.Context) error {
	// Ensure directory exists
	dir := filepath.Dir(s.socketPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create socket dir: %w", err)
	}

	// Remove existing socket file
	os.Remove(s.socketPath)

	// Create listener
	listener, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.socketPath, err)
	}
	s.listener = listener

	// Set permissions (readable/writable by owner only)
	// On Windows, chmod has limited effect but good practice for unix subsys
	os.Chmod(s.socketPath, 0600)

	s.log.Info("MCP server listening", "socket", s.socketPath)

	// Accept connections
	go s.acceptLoop(ctx)

	// Wait for context cancellation
	<-ctx.Done()
	return s.Shutdown()
}

func (s *Server) acceptLoop(ctx context.Context) {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				// Only log strictly necessary errors to avoid noise on graceful shutdown
				// Parse error to see if it's "use of closed network connection"
				s.log.Error("Accept error", "error", err)
				continue
			}
		}

		s.connsMu.Lock()
		s.conns[conn] = struct{}{}
		s.connsMu.Unlock()

		go s.handleConnection(ctx, conn)
	}
}

func (s *Server) handleConnection(ctx context.Context, conn net.Conn) {
	defer func() {
		conn.Close()
		s.connsMu.Lock()
		delete(s.conns, conn)
		s.connsMu.Unlock()
	}()

	s.log.Debug("Client connected")

	reader := bufio.NewReader(conn)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Read line (JSON-RPC message)
		line, err := reader.ReadBytes('\n')
		if err != nil {
			// Expected on disconnect
			s.log.Debug("Client disconnected", "error", err)
			return
		}

		// Parse request
		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			s.sendError(conn, nil, ErrParse, "Parse error")
			continue
		}

		// Handle request
		response := s.handler.Handle(ctx, &req)

		// Send response (if not notification and handler returned something)
		if response != nil && req.ID != nil {
			s.sendResponse(conn, response)
		}
	}
}

func (s *Server) sendResponse(conn net.Conn, resp *Response) {
	data, err := json.Marshal(resp)
	if err != nil {
		s.log.Error("Failed to marshal response", "error", err)
		return
	}

	conn.Write(data)
	conn.Write([]byte("\n"))
}

func (s *Server) sendError(conn net.Conn, id interface{}, code int, message string) {
	resp := &Response{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &Error{Code: code, Message: message},
	}
	s.sendResponse(conn, resp)
}

func (s *Server) Shutdown() error {
	s.log.Info("Shutting down MCP server")

	// Close listener
	if s.listener != nil {
		s.listener.Close()
	}

	// Close all connections
	s.connsMu.Lock()
	for conn := range s.conns {
		conn.Close()
	}
	s.connsMu.Unlock()

	// Remove socket file
	os.Remove(s.socketPath)

	return nil
}

func (s *Server) SocketPath() string {
	return s.socketPath
}
