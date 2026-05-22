package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"nginx-proxy-manager/internal/config"
	"nginx-proxy-manager/internal/model"
)

type ProxyStore interface {
	Load() ([]model.Proxy, error)
	Save(proxies []model.Proxy) error
}

type NginxControl interface {
	TestConfig() (bool, string, error)
	Reload() (string, error)
	Status() (string, string, string)
}

type Handler struct {
	render  *Render
	proxies ProxyStore
	nginx   NginxControl
}

func New(render *Render, proxies ProxyStore, nginx NginxControl) *Handler {
	return &Handler{render: render, proxies: proxies, nginx: nginx}
}

func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	proxies, err := h.proxies.Load()
	if err != nil {
		log.Printf("load proxies: %v", err)
	}
	if proxies == nil {
		proxies = []model.Proxy{}
	}
	h.render.Dashboard(w, map[string]interface{}{
		"Proxies": proxies,
	})
}

func (h *Handler) ListProxies(w http.ResponseWriter, r *http.Request) {
	proxies, err := h.proxies.Load()
	if err != nil {
		log.Printf("load proxies: %v", err)
		h.render.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if proxies == nil {
		proxies = []model.Proxy{}
	}
	h.render.JSON(w, http.StatusOK, proxies)
}

func (h *Handler) CreateProxy(w http.ResponseWriter, r *http.Request) {
	var proxy model.Proxy
	if err := json.NewDecoder(r.Body).Decode(&proxy); err != nil {
		h.render.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if proxy.ID == "" {
		proxy.ID = config.NewProxyID()
	}
	if proxy.Name == "" || proxy.Listen == "" {
		h.render.JSON(w, http.StatusBadRequest, map[string]string{"error": "name and listen are required"})
		return
	}
	if proxy.Protocol == "" {
		proxy.Protocol = "tcp"
	}

	proxies, err := h.proxies.Load()
	if err != nil {
		h.render.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if proxies == nil {
		proxies = []model.Proxy{}
	}
	proxies = append(proxies, proxy)

	if err := h.proxies.Save(proxies); err != nil {
		h.render.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	h.render.JSON(w, http.StatusCreated, proxy)
}

func (h *Handler) UpdateProxy(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.render.JSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
		return
	}

	var updated model.Proxy
	if err := json.NewDecoder(r.Body).Decode(&updated); err != nil {
		h.render.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	proxies, err := h.proxies.Load()
	if err != nil {
		h.render.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if proxies == nil {
		proxies = []model.Proxy{}
	}

	found := false
	for i, p := range proxies {
		if p.ID == id {
			updated.ID = id
			proxies[i] = updated
			found = true
			break
		}
	}
	if !found {
		h.render.JSON(w, http.StatusNotFound, map[string]string{"error": "proxy not found"})
		return
	}

	if err := h.proxies.Save(proxies); err != nil {
		h.render.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	h.render.JSON(w, http.StatusOK, updated)
}

func (h *Handler) DeleteProxy(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.render.JSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
		return
	}

	proxies, err := h.proxies.Load()
	if err != nil {
		h.render.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if proxies == nil {
		proxies = []model.Proxy{}
	}

	filtered := make([]model.Proxy, 0, len(proxies))
	found := false
	for _, p := range proxies {
		if p.ID == id {
			found = true
			continue
		}
		filtered = append(filtered, p)
	}
	if !found {
		h.render.JSON(w, http.StatusNotFound, map[string]string{"error": "proxy not found"})
		return
	}

	if err := h.proxies.Save(filtered); err != nil {
		h.render.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	h.render.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *Handler) ConfigStatus(w http.ResponseWriter, r *http.Request) {
	lastReload, lastTest, testOutput := h.nginx.Status()

	valid, output, err := h.nginx.TestConfig()
	if err != nil {
		h.render.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if output != "" {
		testOutput = output
		lastTest = strings.Split(output, "\n")[0]
	}

	h.render.JSON(w, http.StatusOK, model.ConfigStatus{
		Valid:      valid,
		LastReload: lastReload,
		LastTest:   lastTest,
		TestOutput: testOutput,
	})
}

func (h *Handler) ReloadConfig(w http.ResponseWriter, r *http.Request) {
	output, err := h.nginx.Reload()
	if err != nil {
		h.render.JSON(w, http.StatusInternalServerError, map[string]string{
			"error":  err.Error(),
			"output": output,
		})
		return
	}
	h.render.JSON(w, http.StatusOK, map[string]string{
		"status": "reloaded",
		"output": output,
	})
}
