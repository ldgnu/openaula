// Package api exposes the keyboard configuration over a local HTTP/JSON API.
//
// The server is opt-in: nothing listens unless you run "openaula serve". It can
// also stop itself after a period of inactivity, so on-demand setups consume
// nothing while unused.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ldgnu/openaula/internal/aula"
	"github.com/ldgnu/openaula/internal/device"
	"github.com/ldgnu/openaula/internal/state"
)

// Server is the HTTP API server.
type Server struct {
	svc   *device.Service
	store *state.Store
	mux   *http.ServeMux

	mu       sync.Mutex
	lastUsed time.Time
	idle     time.Duration
	onIdle   func()
}

// New builds a server.
func New(svc *device.Service, store *state.Store) *Server {
	s := &Server{svc: svc, store: store, mux: http.NewServeMux(), lastUsed: time.Now()}
	s.routes()
	return s
}

// SetIdleShutdown makes the server call fn (typically cancel the server
// context) after d with no requests. d <= 0 disables it.
func (s *Server) SetIdleShutdown(d time.Duration, fn func()) {
	s.idle = d
	s.onIdle = fn
}

// Handler returns the HTTP handler.
func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.lastUsed = time.Now()
		s.mu.Unlock()
		s.mux.ServeHTTP(w, r)
	})
}

// Run serves HTTP until ctx is done.
func (s *Server) Run(ctx context.Context, addr string) error {
	srv := &http.Server{Addr: addr, Handler: s.Handler()}

	done := make(chan struct{})
	go func() {
		if s.idle > 0 && s.onIdle != nil {
			t := time.NewTicker(5 * time.Second)
			defer t.Stop()
			for {
				select {
				case <-done:
					return
				case <-t.C:
					s.mu.Lock()
					idleFor := time.Since(s.lastUsed)
					s.mu.Unlock()
					if idleFor > s.idle {
						s.onIdle()
						return
					}
				}
			}
		}
	}()

	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		close(done)
		if err := <-errCh; err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case err := <-errCh:
		close(done)
		return err
	}
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/v1/status", s.handleStatus)
	s.mux.HandleFunc("GET /api/v1/modes", s.handleModes)
	s.mux.HandleFunc("GET /api/v1/keys", s.handleKeys)
	s.mux.HandleFunc("GET /api/v1/state", s.handleGetState)
	s.mux.HandleFunc("PUT /api/v1/state", s.handlePutState)

	s.mux.HandleFunc("POST /api/v1/light", s.handleLight)
	s.mux.HandleFunc("POST /api/v1/off", s.handleOff)
	s.mux.HandleFunc("POST /api/v1/perkey", s.handlePerKey)
	s.mux.HandleFunc("POST /api/v1/remap", s.handleRemap)
	s.mux.HandleFunc("POST /api/v1/remap/reset", s.handleRemapReset)
	s.mux.HandleFunc("POST /api/v1/clock", s.handleClock)
	s.mux.HandleFunc("GET /api/v1/presets", s.handlePresets)
	s.mux.HandleFunc("POST /api/v1/presets/{id}", s.handleApplyPreset)
	s.mux.HandleFunc("GET /api/v1/os", s.handleOSProfiles)
	s.mux.HandleFunc("POST /api/v1/os/{id}", s.handleApplyOS)
	s.mux.HandleFunc("POST /api/v1/lcd/color", s.handleLCDColor)
	s.mux.HandleFunc("POST /api/v1/lcd/image", s.handleLCDImage)

	s.mux.HandleFunc("GET /api/v1/profiles", s.handleProfiles)
	s.mux.HandleFunc("POST /api/v1/profiles/{name}", s.handleSaveProfile)
	s.mux.HandleFunc("POST /api/v1/profiles/{name}/apply", s.handleApplyProfile)
	s.mux.HandleFunc("DELETE /api/v1/profiles/{name}", s.handleDeleteProfile)

	s.mux.HandleFunc("GET /api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
}

// ---- handlers ----

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	st, err := s.svc.Status(s.store)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleModes(w http.ResponseWriter, r *http.Request) {
	type mode struct {
		ID        int    `json:"id"`
		Name      string `json:"name"`
		Color     bool   `json:"color"`
		Speed     bool   `json:"speed"`
		Direction bool   `json:"direction"`
		Rainbow   bool   `json:"rainbow"`
	}
	var out []mode
	for i := 0; i <= 19; i++ {
		m := mode{ID: i, Name: aula.ModeNames[aula.LightingMode(i)]}
		if m.Name == "" {
			continue
		}
		switch aula.LightingMode(i) {
		case aula.ModeOff:
		case aula.ModeStatic:
			m.Color = true
		case aula.ModeColourful, aula.ModeSpectrum:
			m.Speed = true
			m.Rainbow = true
		case aula.ModeScrolling:
			m.Color, m.Speed, m.Direction = true, true, true
		case aula.ModeRolling, aula.ModeRotating, aula.ModeFlowing, aula.ModeTilt:
			m.Color, m.Speed, m.Direction = true, true, true
		default:
			m.Color, m.Speed = true, true
		}
		out = append(out, m)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleKeys(w http.ResponseWriter, r *http.Request) {
	type key struct {
		Name  string `json:"name"`
		Index int    `json:"index"`
		Row   int    `json:"row"`
		Col   int    `json:"col"`
		Width int    `json:"width"`
		Label string `json:"label"`
	}
	names := make(map[uint8]string, len(aula.KeyNameToIndex))
	for n, i := range aula.KeyNameToIndex {
		names[i] = n
	}
	var out []key
	for row, rowKeys := range aula.KeyLayout {
		col := 0
		for _, k := range rowKeys {
			out = append(out, key{
				Name:  k.Name,
				Index: int(aula.KeyNameToIndex[k.Name]),
				Row:   row,
				Col:   col,
				Width: k.Width,
				Label: k.Label,
			})
			col += k.Width
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleGetState(w http.ResponseWriter, r *http.Request) {
	st, err := s.store.Load()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handlePutState(w http.ResponseWriter, r *http.Request) {
	var st state.Settings
	if err := decodeJSON(r, &st); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := s.store.Save(st); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleLight(w http.ResponseWriter, r *http.Request) {
	var l state.Lighting
	if err := decodeJSON(r, &l); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := s.svc.ApplyLighting(l); err != nil {
		writeDeviceErr(w, err)
		return
	}
	st, _ := s.store.Load()
	st.Lighting = l
	_ = s.store.Save(st)
	writeJSON(w, http.StatusOK, l)
}

func (s *Server) handleOff(w http.ResponseWriter, r *http.Request) {
	l := state.Lighting{Mode: 0}
	if err := s.svc.ApplyLighting(l); err != nil {
		writeDeviceErr(w, err)
		return
	}
	st, _ := s.store.Load()
	st.Lighting = l
	_ = s.store.Save(st)
	writeJSON(w, http.StatusOK, map[string]string{"result": "off"})
}

func (s *Server) handlePerKey(w http.ResponseWriter, r *http.Request) {
	var pk state.PerKey
	if err := decodeJSON(r, &pk); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if pk.Brightness == 0 {
		pk.Brightness = 5
	}
	if err := s.svc.ApplyPerKey(pk); err != nil {
		writeDeviceErr(w, err)
		return
	}
	st, _ := s.store.Load()
	st.PerKey = pk
	_ = s.store.Save(st)
	writeJSON(w, http.StatusOK, pk)
}

func (s *Server) handleRemap(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Layer string            `json:"layer"`
		Keys  map[string]string `json:"keys"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := s.svc.ApplyRemap(req.Layer, req.Keys); err != nil {
		writeDeviceErr(w, err)
		return
	}
	st, _ := s.store.Load()
	if strings.EqualFold(req.Layer, "fn") {
		st.Remap.FN = req.Keys
	} else {
		st.Remap.Normal = req.Keys
	}
	_ = s.store.Save(st)
	writeJSON(w, http.StatusOK, req)
}

func (s *Server) handleRemapReset(w http.ResponseWriter, r *http.Request) {
	layer := r.URL.Query().Get("layer")
	if layer == "" {
		layer = "normal"
	}
	if err := s.svc.ResetRemap(layer); err != nil {
		writeDeviceErr(w, err)
		return
	}
	st, _ := s.store.Load()
	if strings.EqualFold(layer, "fn") {
		st.Remap.FN = map[string]string{}
	} else {
		st.Remap.Normal = map[string]string{}
	}
	_ = s.store.Save(st)
	writeJSON(w, http.StatusOK, map[string]string{"result": "reset"})
}

func (s *Server) handlePresets(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, device.Presets())
}

func (s *Server) handleApplyPreset(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	keys, err := s.svc.ApplyPreset(id, true)
	if err != nil {
		writeDeviceErr(w, err)
		return
	}
	st, _ := s.store.Load()
	for k, v := range keys {
		st.Remap.Normal[k] = v
	}
	_ = s.store.Save(st)
	writeJSON(w, http.StatusOK, map[string]any{"applied": id, "keys": keys})
}

func (s *Server) handleOSProfiles(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, device.OSProfiles())
}

func (s *Server) handleApplyOS(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	st, err := s.store.Load()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	keys, err := s.svc.ApplyOSProfile(id, st.Remap.Normal)
	if err != nil {
		writeDeviceErr(w, err)
		return
	}
	st.Remap.Normal = keys
	st.OS = id
	_ = s.store.Save(st)
	writeJSON(w, http.StatusOK, map[string]any{"applied": id, "keys": keys})
}

func (s *Server) handleClock(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.SyncClock(); err != nil {
		writeDeviceErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"result": "synced", "time": time.Now().Format(time.RFC3339)})
}

func (s *Server) handleLCDColor(w http.ResponseWriter, r *http.Request) {
	var c struct {
		R uint8 `json:"r"`
		G uint8 `json:"g"`
		B uint8 `json:"b"`
	}
	if err := decodeJSON(r, &c); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := s.svc.LCDSolid(c.R, c.G, c.B); err != nil {
		writeDeviceErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) handleLCDImage(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("missing form field 'file': %w", err))
		return
	}
	defer file.Close()

	// Uploading can corrupt the built-in LCD menus, so it needs an explicit flag.
	if r.FormValue("confirm") != "true" {
		writeErr(w, http.StatusPreconditionRequired, fmt.Errorf("LCD uploads can corrupt the built-in screen graphics; send confirm=true to proceed"))
		return
	}

	if err := s.svc.LCDImage(file, nil); err != nil {
		writeDeviceErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"result": "uploaded"})
}

func (s *Server) handleProfiles(w http.ResponseWriter, r *http.Request) {
	names, err := s.store.ListProfiles()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, names)
}

func (s *Server) handleSaveProfile(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	st, err := s.store.Load()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if err := s.store.SaveProfile(name, st); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"saved": name})
}

func (s *Server) handleApplyProfile(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	st, err := s.store.LoadProfile(name)
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	if st.Lighting.Mode > 0 {
		if err := s.svc.ApplyLighting(st.Lighting); err != nil {
			writeDeviceErr(w, err)
			return
		}
	}
	if len(st.PerKey.Colors) > 0 {
		if err := s.svc.ApplyPerKey(st.PerKey); err != nil {
			writeDeviceErr(w, err)
			return
		}
	}
	if len(st.Remap.Normal) > 0 {
		if err := s.svc.ApplyRemap("normal", st.Remap.Normal); err != nil {
			writeDeviceErr(w, err)
			return
		}
	}
	if len(st.Remap.FN) > 0 {
		if err := s.svc.ApplyRemap("fn", st.Remap.FN); err != nil {
			writeDeviceErr(w, err)
			return
		}
	}
	if err := s.store.Save(st); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleDeleteProfile(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := s.store.DeleteProfile(name); err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"deleted": name})
}

// ---- helpers ----

func decodeJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(io.LimitReader(r.Body, 8<<20))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func writeErr(w http.ResponseWriter, code int, err error) {
	writeJSON(w, code, map[string]string{"error": err.Error()})
}

func writeDeviceErr(w http.ResponseWriter, err error) {
	log.Printf("device error: %v", err)
	code := http.StatusInternalServerError
	if errors.Is(err, device.ErrNotConfigured) {
		code = http.StatusConflict
	}
	writeErr(w, code, err)
}
