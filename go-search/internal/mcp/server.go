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
	addr     string
	network  string
	handler  *Handler
	listener net.Listener

	// Active connections
	connsMu sync.RWMutex
	conns   map[net.Conn]struct{}

	log *slog.Logger
}

type ServerConfig struct {
	SocketPath string
	TCPAddr    string
	Handler    *Handler
}

func NewServer(cfg ServerConfig) *Server {
	network := "unix"
	addr := cfg.SocketPath

	if cfg.TCPAddr != "" {
		network = "tcp"
		addr = cfg.TCPAddr
	} else if addr == "" {
		home, _ := os.UserHomeDir()
		// Use a platform-appropriate directory
		addr = filepath.Join(home, ".local", "run", "rice-search", "mcp.sock")
	}

	return &Server{
		addr:    addr,
		network: network,
		handler: cfg.Handler,
		conns:   make(map[net.Conn]struct{}),
		log:     slog.Default().With("component", "mcp"),
	}
}

func (s *Server) Start(ctx context.Context) error {
	var listener net.Listener
	var err error

	if s.network == "unix" {
		// Ensure directory exists
		dir := filepath.Dir(s.addr)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create socket dir: %w", err)
		}

		// Remove existing socket file
		os.Remove(s.addr)

		listener, err = net.Listen("unix", s.addr)
		if err != nil {
			return fmt.Errorf("failed to listen on %s: %w", s.addr, err)
		}

		// Set permissions
		os.Chmod(s.addr, 0600)
	} else {
		listener, err = net.Listen("tcp", s.addr)
		if err != nil {
			return fmt.Errorf("failed to listen on %s: %w", s.addr, err)
		}
	}

	s.listener = listener
	s.log.Info("MCP server listening", "network", s.network, "addr", s.addr)

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

	// Remove socket file if unix
	if s.network == "unix" {
		os.Remove(s.addr)
	}

	return nil
}

func (s *Server) SocketPath() string {
	return s.addr
}
