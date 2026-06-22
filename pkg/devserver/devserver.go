package devserver

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gorilla/websocket"

	"github.com/techblog/staticgen/pkg/builder"
	"github.com/techblog/staticgen/pkg/config"
	"github.com/techblog/staticgen/pkg/logger"
)

type DevServer struct {
	cfg       *config.Config
	log       *logger.Logger
	builder   *builder.Builder
	watcher   *fsnotify.Watcher
	upgrader  websocket.Upgrader
	clients   map[*websocket.Conn]bool
	clientsMu sync.RWMutex
	reloadCh  chan struct{}
	stopCh    chan struct{}
}

func New(cfg *config.Config, log *logger.Logger, b *builder.Builder) (*DevServer, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("create watcher: %w", err)
	}

	return &DevServer{
		cfg:      cfg,
		log:      log,
		builder:  b,
		watcher:  w,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
		clients:  make(map[*websocket.Conn]bool),
		reloadCh: make(chan struct{}, 1),
		stopCh:   make(chan struct{}),
	}, nil
}

func (s *DevServer) Start() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Dev.Host, s.cfg.Dev.Port)

	if err := s.setupWatchers(); err != nil {
		return err
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/__ws", s.handleWebSocket)

	fileServer := http.FileServer(http.Dir(s.cfg.Paths.Public))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/" {
			path = "/index.html"
		}

		if filepath.Ext(path) == "" {
			if s.cfg.Build.PrettyURLs {
				path = filepath.Clean(path) + "/index.html"
			} else {
				path = path + ".html"
			}
		}

		s.log.Debug("Serving: %s", path)
		r.URL.Path = path
		fileServer.ServeHTTP(w, r)
	})

	go s.watchLoop()
	go s.reloadLoop()

	s.log.Success("Dev server running at http://%s", addr)
	s.log.Info("Watching for changes...")

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	return server.ListenAndServe()
}

func (s *DevServer) setupWatchers() error {
	dirs := []string{
		s.cfg.Paths.Content,
		s.cfg.Paths.Static,
		s.cfg.Paths.Templates,
	}

	for _, dir := range dirs {
		if err := s.addDirToWatcher(dir); err != nil {
			s.log.Warn("Failed to watch %s: %v", dir, err)
		}
	}

	return nil
}

func (s *DevServer) addDirToWatcher(dir string) error {
	if err := s.watcher.Add(dir); err != nil {
		return err
	}
	s.log.Debug("Watching directory: %s", dir)

	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() && path != dir {
			if err := s.watcher.Add(path); err != nil {
				s.log.Debug("Failed to watch subdir %s: %v", path, err)
			}
		}
		return nil
	})
}

func (s *DevServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.log.Error("WebSocket upgrade failed: %v", err)
		return
	}

	s.clientsMu.Lock()
	s.clients[conn] = true
	s.clientsMu.Unlock()

	s.log.Debug("WebSocket client connected")

	defer func() {
		s.clientsMu.Lock()
		delete(s.clients, conn)
		s.clientsMu.Unlock()
		conn.Close()
		s.log.Debug("WebSocket client disconnected")
	}()

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			return
		}
	}
}

func (s *DevServer) watchLoop() {
	var debounceTimer *time.Timer

	for {
		select {
		case event, ok := <-s.watcher.Events:
			if !ok {
				return
			}

			if event.Op&fsnotify.Chmod == fsnotify.Chmod {
				continue
			}

			s.log.Info("File changed: %s (%s)", event.Name, event.Op)

			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			debounceTimer = time.AfterFunc(300*time.Millisecond, func() {
				select {
				case s.reloadCh <- struct{}{}:
				default:
				}
			})

		case err, ok := <-s.watcher.Errors:
			if !ok {
				return
			}
			s.log.Error("Watcher error: %v", err)

		case <-s.stopCh:
			return
		}
	}
}

func (s *DevServer) reloadLoop() {
	for {
		select {
		case <-s.reloadCh:
			s.log.Info("Rebuilding...")
			start := time.Now()

			s.cfg.Build.Incremental = false
			_, err := s.builder.Build(false)
			if err != nil {
				s.log.Error("Build failed: %v", err)
				continue
			}

			duration := time.Since(start)
			s.log.Success("Rebuilt in %v", duration)

			s.broadcastReload()

		case <-s.stopCh:
			return
		}
	}
}

func (s *DevServer) broadcastReload() {
	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()

	s.log.Debug("Broadcasting reload to %d clients", len(s.clients))

	for conn := range s.clients {
		err := conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"reload"}`))
		if err != nil {
			s.log.Debug("Failed to send reload: %v", err)
		}
	}
}

func (s *DevServer) Stop() error {
	close(s.stopCh)
	return s.watcher.Close()
}

func GetHotReloadScript() string {
	return `
<script>
(function() {
    var protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
    var wsUrl = protocol + '//' + location.host + '/__ws';
    var ws;
    var reconnectDelay = 1000;
    var maxReconnectDelay = 5000;

    function connect() {
        ws = new WebSocket(wsUrl);

        ws.onopen = function() {
            console.log('[staticgen] Hot reload connected');
            reconnectDelay = 1000;
        };

        ws.onmessage = function(event) {
            try {
                var data = JSON.parse(event.data);
                if (data.type === 'reload') {
                    console.log('[staticgen] Reloading...');
                    location.reload();
                }
            } catch(e) {
                console.error('[staticgen] Invalid message:', e);
            }
        };

        ws.onclose = function() {
            console.log('[staticgen] Connection lost, reconnecting...');
            setTimeout(connect, reconnectDelay);
            reconnectDelay = Math.min(reconnectDelay * 2, maxReconnectDelay);
        };

        ws.onerror = function(err) {
            console.error('[staticgen] WebSocket error:', err);
        };
    }

    connect();
})();
</script>
`
}
