package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"litl/lib/api"
	"litl/lib/clipboard"
	"litl/lib/config"
	"litl/lib/notifier"
)

type Server struct {
	cfg      *config.Config
	port     int
	httpSrv  *http.Server
	client   *api.OpenRouterClient
	clip     clipboard.Clipboard
	notifier notifier.Notifier
}

func New(cfg *config.Config, port int) *Server {
	s := &Server{
		cfg:      cfg,
		port:     port,
		client:   api.NewOpenRouterClient(cfg.APIKey),
		clip:     clipboard.New(),
		notifier: notifier.New(),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/process", s.handleProcess)

	s.httpSrv = &http.Server{
		Addr:         fmt.Sprintf("127.0.0.1:%d", port),
		Handler:      s.authMiddleware(mux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	return s
}

func (s *Server) Start() error {
	return s.httpSrv.ListenAndServe()
}

func (s *Server) Stop(ctx context.Context) error {
	return s.httpSrv.Shutdown(ctx)
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		if token != "Bearer "+s.cfg.AuthToken {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	resp := map[string]string{"status": "ok"}
	json.NewEncoder(w).Encode(resp)
}

type ProcessRequest struct {
	Action string `json:"action"`
	Text   string `json:"text"`
}

type ProcessResponse struct {
	Result string `json:"result"`
	Error  string `json:"error,omitempty"`
}

func (s *Server) handleProcess(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ProcessRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Получаем текст
	text := req.Text
	if text == "" {
		var err error
		text, err = s.clip.Get()
		if err != nil {
			s.sendError(w, "Failed to read clipboard: "+err.Error())
			return
		}
	}

	if text == "" {
		s.sendError(w, "No text to process")
		return
	}

	// Ищем action в конфиге
	actionCfg, ok := s.cfg.Actions[req.Action]
	if !ok {
		s.sendError(w, fmt.Sprintf("Unknown action: %s", req.Action))
		return
	}

	log.Printf("Processing action '%s' with %d chars", req.Action, len(text))
	s.notifier.Notify("AI Assistant", "Processing...")

	// Загружаем контекст
	contextText := ""
	if actionCfg.ContextFile != "" {
		var err error
		contextText, err = s.cfg.LoadContext(actionCfg.ContextFile)
		if err != nil {
			log.Printf("Warning: failed to load context: %v", err)
		}
	}

	// Вызов API с контекстом для отмены
	result, err := s.client.Chat(r.Context(), api.ChatRequest{
		Model:        actionCfg.Model,
		SystemPrompt: actionCfg.SystemPrompt,
		Context:      contextText,
		UserMessage:  text,
		Temperature:  actionCfg.Temperature,
		MaxTokens:    actionCfg.MaxTokens,
	})
	if err != nil {
		s.sendError(w, "API error: "+err.Error())
		s.notifier.Notify("Error", err.Error())
		return
	}

	// Сохраняем в буфер
	if err := s.clip.Set(result); err != nil {
		log.Printf("Warning: failed to set clipboard: %v", err)
	}

	s.notifier.Notify("Done", fmt.Sprintf("Response (%d chars) copied", len(result)))

	json.NewEncoder(w).Encode(ProcessResponse{Result: result})
}

func (s *Server) sendError(w http.ResponseWriter, msg string) {
	log.Printf("Error: %s", msg)
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(ProcessResponse{Error: msg})
}

var _ net.Listener
