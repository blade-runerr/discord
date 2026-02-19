package httpserver

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"mini-discord/internal/storage"
	"mini-discord/internal/voicews"
	"mini-discord/internal/websocket"
)

type Server struct {
	addr      string
	mux       *http.ServeMux
	templates *template.Template
	hub       *websocket.Hub
	voiceHub  *voicews.Hub
	store     *storage.Store
}

func NewServer(addr string) (*Server, error) {
	mux := http.NewServeMux()

	store, err := storage.NewStore(filepath.Join("data", "chat.db"))
	if err != nil {
		return nil, err
	}

	hub := websocket.NewHub(store)
	go hub.Run()

	voiceHub := voicews.NewHub()
	go voiceHub.Run()

	s := &Server{
		addr:     addr,
		mux:      mux,
		hub:      hub,
		voiceHub: voiceHub,
		store:    store,
	}

	if err := s.loadTemplates(); err != nil {
		return nil, err
	}
	s.registerRoutes()
	return s, nil
}

func (s *Server) loadTemplates() error {
	pattern := filepath.Join("web", "templates", "*.html")
	tpl, err := template.ParseGlob(pattern)
	if err != nil {
		return err
	}
	s.templates = tpl
	return nil
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("/", s.handleIndex)

	staticDir := http.Dir(filepath.Join("web", "static"))
	fileServer := http.FileServer(staticDir)
	s.mux.Handle("/static/", http.StripPrefix("/static/", fileServer))

	s.mux.HandleFunc("/ws", s.handleWebSocket)
	s.mux.HandleFunc("/ws/voice", s.handleVoiceWebSocket)
	s.mux.HandleFunc("/api/history", s.handleHistory)
	s.mux.HandleFunc("/api/webrtc-config", s.handleWebRTCConfig)
}

func (s *Server) Start() error {
	log.Printf("server started on %s", s.addr)
	return http.ListenAndServe(s.addr, s.mux)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if err := s.templates.ExecuteTemplate(w, "index.html", nil); err != nil {
		log.Printf("execute template: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	websocket.ServeWS(s.hub, w, r)
}

func (s *Server) handleVoiceWebSocket(w http.ResponseWriter, r *http.Request) {
	voicews.ServeWS(s.voiceHub, w, r)
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	channel := r.URL.Query().Get("channel")
	if channel == "" {
		channel = "general"
	}

	limit := 50
	if limitRaw := r.URL.Query().Get("limit"); limitRaw != "" {
		if parsed, err := strconv.Atoi(limitRaw); err == nil && parsed > 0 && parsed <= 200 {
			limit = parsed
		}
	}

	messages, err := s.store.RecentMessages(channel, limit)
	if err != nil {
		log.Printf("history read error: %v", err)
		http.Error(w, "failed to load history", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(messages); err != nil {
		log.Printf("history write error: %v", err)
	}
}

type iceServer struct {
	URLs       []string `json:"urls"`
	Username   string   `json:"username,omitempty"`
	Credential string   `json:"credential,omitempty"`
}

type webrtcConfigResponse struct {
	ICEServers []iceServer `json:"iceServers"`
}

func (s *Server) handleWebRTCConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	iceServers := []iceServer{
		{URLs: []string{"stun:stun.l.google.com:19302"}},
	}

	turnHost := strings.TrimSpace(os.Getenv("TURN_HOST"))
	turnUser := strings.TrimSpace(os.Getenv("TURN_USERNAME"))
	turnPass := os.Getenv("TURN_PASSWORD")
	turnPort := strings.TrimSpace(os.Getenv("TURN_PORT"))
	if turnPort == "" {
		turnPort = "3478"
	}

	if turnHost != "" && turnUser != "" && turnPass != "" {
		iceServers = append(iceServers, iceServer{
			URLs: []string{
				"turn:" + turnHost + ":" + turnPort + "?transport=udp",
				"turn:" + turnHost + ":" + turnPort + "?transport=tcp",
			},
			Username:   turnUser,
			Credential: turnPass,
		})
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(webrtcConfigResponse{ICEServers: iceServers}); err != nil {
		log.Printf("webrtc config write error: %v", err)
	}
}
