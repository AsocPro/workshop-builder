package management

import (
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/asocpro/workshop-builder/cli/docker"
)

// Step is a minimal step descriptor used in the management UI.
type Step struct {
	ID    string
	Title string
}

// Config holds all parameters for creating a Server.
type Config struct {
	Port         int
	WorkshopPort int
	MgmtURL      string // this server's URL (used when starting replacement containers)
	WorkshopURL  string // URL of the workshop frontend, shown as a link
	DC           *docker.Client
	Image        string // full step image ref (e.g. "localhost/hello-linux:step-1-intro")
	Steps        []Step
	CurrentStep  string // step ID currently running (e.g. "step-1-intro")
}

// Server is the host-side management HTTP server.
type Server struct {
	mu            sync.Mutex
	port          int
	workshopPort  int
	mgmtURL       string
	workshopURL   string
	dc            *docker.Client
	workshopImage string // base image (e.g. "localhost/hello-linux")
	currentID     string
	currentStep   string
	steps         []Step
}

// NewServer creates (but does not start) the management server.
func NewServer(cfg Config) *Server {
	baseImage := cfg.Image
	if idx := lastColon(cfg.Image); idx != -1 {
		baseImage = cfg.Image[:idx]
	}
	steps := make([]Step, len(cfg.Steps))
	copy(steps, cfg.Steps)
	return &Server{
		port:          cfg.Port,
		workshopPort:  cfg.WorkshopPort,
		mgmtURL:       cfg.MgmtURL,
		workshopURL:   cfg.WorkshopURL,
		dc:            cfg.DC,
		workshopImage: baseImage,
		currentStep:   cfg.CurrentStep,
		steps:         steps,
	}
}

func lastColon(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == ':' {
			return i
		}
	}
	return -1
}

// SetCurrentContainer records the running container ID.
func (s *Server) SetCurrentContainer(containerID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentID = containerID
}

// SetCurrentStep records which step is currently running.
func (s *Server) SetCurrentStep(stepID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentStep = stepID
}

// CurrentContainer returns the current container ID.
func (s *Server) CurrentContainer() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.currentID
}

// Start starts the management HTTP server in a goroutine.
func (s *Server) Start() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/step/", s.handleGoToStep)
	mux.HandleFunc("/status", s.handleStatus)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: mux,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("management server error: %v", err)
		}
	}()
}
