package config

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"nginx-proxy-manager/internal/model"
)

type Manager struct {
	mu       sync.Mutex
	confPath string
	entries  []StreamEntry
}

func NewManager(confPath string) *Manager {
	return &Manager{confPath: confPath}
}

func (m *Manager) Load() ([]model.Proxy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.confPath)
	if err != nil {
		if os.IsNotExist(err) {
			m.entries = nil
			return nil, nil
		}
		return nil, fmt.Errorf("read nginx.conf: %w", err)
	}

	content := string(data)
	before, block, after := findStreamBlock(content)
	_ = before
	_ = after

	if block == "" {
		m.entries = nil
		return nil, nil
	}

	inner := extractBlockContent(block)
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

	data, err := os.ReadFile(m.confPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Create new file with stream block
			newContent := buildFullConfig(proxies)
			return os.WriteFile(m.confPath, []byte(newContent), 0644)
		}
		return fmt.Errorf("read nginx.conf: %w", err)
	}

	content := string(data)
	before, block, after := findStreamBlock(content)

	newInner := BuildStreamContent(m.entries, proxies)
	indented := indentBlock(newInner)
	newBlock := "stream {\n" + indented + "\n}"

	var newContent string
	if block == "" {
		newContent = strings.TrimRight(content, "\n ") + "\n\n" + newBlock + "\n"
	} else {
		newContent = before + newBlock + after
	}

	return os.WriteFile(m.confPath, []byte(newContent), 0644)
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

func findStreamBlock(content string) (before, block, after string) {
	idx := strings.Index(content, "stream")
	if idx < 0 {
		return content, "", ""
	}

	braceOffset := strings.IndexByte(content[idx:], '{')
	if braceOffset < 0 {
		return content[:idx], "", content[idx:]
	}
	braceStart := idx + braceOffset

	depth := 1
	i := braceStart + 1
	for i < len(content) && depth > 0 {
		switch content[i] {
		case '{':
			depth++
		case '}':
			depth--
		}
		i++
	}
	if depth != 0 {
		return content, "", ""
	}

	blockEnd := i - 1
	before = content[:idx]
	block = content[idx : blockEnd+1]
	after = content[blockEnd+1:]
	return before, block, after
}

func buildFullConfig(proxies []model.Proxy) string {
	var b strings.Builder
	b.WriteString("worker_processes auto;\n")
	b.WriteString("events {\n    worker_connections 1024;\n}\n\n")
	b.WriteString("stream {\n")
	for _, p := range proxies {
		name := sanitizeUpstreamName(p.Name)
		if len(p.Backends) > 0 {
			b.WriteString("    ")
			b.WriteString(buildUpstreamBlock(p, name))
			b.WriteString("\n\n")
		}
		b.WriteString("    ")
		b.WriteString(buildServerBlock(p, name))
		b.WriteString("\n")
	}
	b.WriteString("}")
	return b.String()
}
