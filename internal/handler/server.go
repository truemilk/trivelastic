package handler

import (
	"net/http"

	"github.com/rs/zerolog"
	"github.com/truemilk/trivelastic/internal/config"
	"github.com/truemilk/trivelastic/internal/elasticsearch"
	"github.com/truemilk/trivelastic/internal/logger"
	"github.com/truemilk/trivelastic/internal/worker"
)

type Server struct {
	cfg        *config.Config
	workerPool *worker.Pool
	log        zerolog.Logger
}

func NewServer(cfg *config.Config, pool *worker.Pool) *Server {
	return &Server{
		cfg:        cfg,
		workerPool: pool,
		log:        logger.GetLogger("server"),
	}
}

func (s *Server) Start() error {
	// Create Elasticsearch client
	s.log.Info().
		Str("es_url", s.cfg.ES.URL).
		Str("es_index", s.cfg.ES.Index).
		Msg("Initializing Elasticsearch client")

	esClient := elasticsearch.NewClient(&s.cfg.ES)
	s.workerPool.SetElasticsearchClient(esClient)

	// Set up the HTTP server with the concurrent handler
	http.HandleFunc("/", s.handleRequest)

	s.log.Info().
		Str("port", s.cfg.Port).
		Msg("Starting HTTP server")

	if err := http.ListenAndServe(":"+s.cfg.Port, nil); err != nil {
		s.log.Error().
			Err(err).
			Str("port", s.cfg.Port).
			Msg("Failed to start HTTP server")
		return err
	}

	return nil
}

func (s *Server) handleRequest(w http.ResponseWriter, r *http.Request) {
	s.log.Debug().
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("remote_addr", r.RemoteAddr).
		Msg("Handling incoming request")

	s.workerPool.Submit(w, r)
}
