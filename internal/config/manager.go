package config

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"nginx-proxy-manager/internal/model"
)

type Manager struct {
	mu         sync.Mutex
	streamPath string
	entries    []StreamEntry
}

func NewManager(streamPath string) *Manager {
	return &Manager{streamPath: streamPath}
}

func (m *Manager) Load() ([]model.Proxy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.streamPath)
	if err != nil {
		if os.IsNotExist(err) {
			m.entries = nil
			return nil, nil
		}
		return nil, fmt.Errorf("read stream config: %w", err)
	}

	inner := extractBlockContent(string(data))
	if inner == "" {
		m.entries = nil
		return nil, nil
	}

	result, err := ParseStreamBlock(inner)
	if err != nil {
		return nil, fmt.Errorf("parse stream block: %w", err)
	}

	m.entries = result.Entries
	return result.ToProxies(), nil
}

func (m *Manager) Save(proxies []model.Proxy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	newInner := BuildStreamContent(m.entries, proxies)
	indented := indentBlock(newInner)
	newContent := "stream {\n" + indented + "\n}\n"

	dir := m.streamPath
	if idx := strings.LastIndexByte(dir, '/'); idx >= 0 {
		dir = dir[:idx]
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	return os.WriteFile(m.streamPath, []byte(newContent), 0644)
}

func indentBlock(s string) string {
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) != "" {
			lines[i] = "    " + line
		}
	}
	return strings.Join(lines, "\n")
}

func extractBlockContent(block string) string {
	braceIdx := strings.IndexByte(block, '{')
	if braceIdx < 0 {
		return ""
	}
	endIdx := strings.LastIndexByte(block, '}')
	if endIdx < 0 {
		return ""
	}
	return strings.TrimSpace(block[braceIdx+1 : endIdx])
}
