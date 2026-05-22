package config

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"

	"nginx-proxy-manager/internal/model"
)

type BlockType int

const (
	BlockRaw BlockType = iota
	BlockUpstream
	BlockServer
)

type StreamEntry struct {
	Type     BlockType
	Raw      string         // for BlockRaw
	ProxyID  string         // for BlockUpstream/BlockServer
	Upstream *UpstreamEntry // for BlockUpstream
	Server   *ServerEntry   // for BlockServer
}

type UpstreamEntry struct {
	Name           string
	Backends       []model.Backend
	BalanceMethod  string
	HashConsistent bool
}

type ServerEntry struct {
	Listen         string
	IsUDP          bool
	ProxyPass      string
	ProxyProtocol  bool
	ProxyTimeout   string
	ConnectTimeout string
	TCPKeepalive   bool
	TCPNodelay     bool
}

type ParseResult struct {
	Entries []StreamEntry
}

var proxyIDRe = regexp.MustCompile(`#\s*nginx-proxy-manager-id:\s*(\S+)`)

func NewProxyID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func splitTopLevel(s string) []string {
	var parts []string
	depth := 0
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth < 0 {
				depth = 0
			}
			if depth == 0 {
				parts = append(parts, s[start:i+1])
				start = i + 1
			}
		case ';':
			if depth == 0 {
				parts = append(parts, s[start:i+1])
				start = i + 1
			}
		}
	}
	rem := strings.TrimSpace(s[start:])
	if rem != "" {
		parts = append(parts, rem)
	}
	return parts
}

func ParseStreamBlock(content string) (*ParseResult, error) {
	result := &ParseResult{}
	content = strings.TrimSpace(content)
	if content == "" {
		return result, nil
	}
	statements := splitTopLevel(content)
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		first := firstDirectiveLine(stmt)
		if strings.HasPrefix(first, "upstream ") && strings.HasSuffix(stmt, "}") {
			entry, err := parseUpstreamBlock(stmt)
			if err != nil {
				result.Entries = append(result.Entries, StreamEntry{Type: BlockRaw, Raw: stmt})
				continue
			}
			result.Entries = append(result.Entries, *entry)
		} else if strings.HasPrefix(first, "server") && strings.HasSuffix(stmt, "}") {
			entry, err := parseServerBlock(stmt)
			if err != nil {
				result.Entries = append(result.Entries, StreamEntry{Type: BlockRaw, Raw: stmt})
				continue
			}
			result.Entries = append(result.Entries, *entry)
		} else {
			result.Entries = append(result.Entries, StreamEntry{Type: BlockRaw, Raw: stmt})
		}
	}
	return result, nil
}

func firstDirectiveLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		return trimmed
	}
	return ""
}

func skipCommentLines(s string) string {
	lines := strings.Split(s, "\n")
	var out []string
	started := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !started {
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}
			started = true
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func extractIDComment(s string) string {
	matches := proxyIDRe.FindStringSubmatch(s)
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}

func stripComments(s string) string {
	lines := strings.Split(s, "\n")
	var result []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		result = append(result, line)
	}
	return strings.Join(result, "\n")
}

func parseUpstreamBlock(stmt string) (*StreamEntry, error) {
	proxyID := extractIDComment(stmt)
	if proxyID == "" {
		proxyID = NewProxyID()
	}

	rest := skipCommentLines(stmt)
	rest = strings.TrimSpace(rest)
	if !strings.HasPrefix(rest, "upstream ") {
		return nil, fmt.Errorf("not an upstream block")
	}
	rest = strings.TrimPrefix(rest, "upstream ")
	rest = strings.TrimSpace(rest)

	// Extract name (first word before {)
	braceIdx := strings.IndexByte(rest, '{')
	if braceIdx < 0 {
		return nil, fmt.Errorf("no opening brace")
	}
	name := strings.TrimSpace(rest[:braceIdx])
	// Remove any trailing comment syntax from name
	name = strings.Split(name, "#")[0]
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("empty upstream name")
	}

	// Extract body between { }
	body := rest[braceIdx+1 : strings.LastIndexByte(rest, '}')]
	body = stripComments(body)
	body = strings.TrimSpace(body)

	entry := &UpstreamEntry{Name: name}
	lines := splitTopLevel(body)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "server ") {
			backend, err := parseServerDirective(line)
			if err != nil {
				continue
			}
			entry.Backends = append(entry.Backends, backend)
		} else if line == "least_conn;" {
			entry.BalanceMethod = "least_conn"
		} else if line == "random;" {
			entry.BalanceMethod = "random"
		} else if strings.HasPrefix(line, "hash ") {
			entry.BalanceMethod = "hash"
			if strings.Contains(line, "consistent") {
				entry.HashConsistent = true
			}
		} else if line == "round_robin;" {
			entry.BalanceMethod = "round_robin"
		}
	}

	return &StreamEntry{
		Type:     BlockUpstream,
		ProxyID:  proxyID,
		Upstream: entry,
	}, nil
}

func parseServerDirective(line string) (model.Backend, error) {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "server ")
	line = strings.TrimSuffix(line, ";")
	line = strings.TrimSpace(line)

	parts := strings.Fields(line)
	if len(parts) == 0 {
		return model.Backend{}, fmt.Errorf("empty server directive")
	}

	b := model.Backend{Address: parts[0]}
	for i := 1; i < len(parts); i++ {
		opt := parts[i]
		switch {
		case opt == "backup":
			b.Backup = true
		case strings.HasPrefix(opt, "weight="):
			b.Weight, _ = strconv.Atoi(strings.TrimPrefix(opt, "weight="))
		case strings.HasPrefix(opt, "max_fails="):
			b.MaxFails, _ = strconv.Atoi(strings.TrimPrefix(opt, "max_fails="))
		case strings.HasPrefix(opt, "fail_timeout="):
			b.FailTimeout = strings.TrimPrefix(opt, "fail_timeout=")
		case strings.HasPrefix(opt, "slow_start="):
			b.SlowStart = strings.TrimPrefix(opt, "slow_start=")
		case strings.HasPrefix(opt, "max_conns="):
			b.MaxConns, _ = strconv.Atoi(strings.TrimPrefix(opt, "max_conns="))
		}
	}
	return b, nil
}

func parseServerBlock(stmt string) (*StreamEntry, error) {
	proxyID := extractIDComment(stmt)
	if proxyID == "" {
		proxyID = NewProxyID()
	}

	rest := skipCommentLines(stmt)
	rest = strings.TrimSpace(rest)
	if !strings.HasPrefix(rest, "server") {
		return nil, fmt.Errorf("not a server block")
	}

	braceIdx := strings.IndexByte(rest, '{')
	if braceIdx < 0 {
		return nil, fmt.Errorf("no opening brace")
	}
	body := rest[braceIdx+1 : strings.LastIndexByte(rest, '}')]
	body = stripComments(body)
	body = strings.TrimSpace(body)

	entry := &ServerEntry{}
	lines := splitTopLevel(body)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		trimmed := strings.TrimSuffix(line, ";")
		trimmed = strings.TrimSpace(trimmed)

		switch {
		case strings.HasPrefix(trimmed, "listen "):
			listenVal := strings.TrimPrefix(trimmed, "listen ")
			listenVal = strings.TrimSpace(listenVal)
			fields := strings.Fields(listenVal)
			if len(fields) > 0 {
				entry.Listen = fields[0]
				for _, f := range fields[1:] {
					if f == "udp" {
						entry.IsUDP = true
					}
				}
			}
		case strings.HasPrefix(trimmed, "proxy_pass "):
			entry.ProxyPass = strings.TrimSpace(strings.TrimPrefix(trimmed, "proxy_pass "))
		case trimmed == "proxy_protocol on":
			entry.ProxyProtocol = true
		case trimmed == "proxy_protocol off":
			entry.ProxyProtocol = false
		case strings.HasPrefix(trimmed, "proxy_timeout "):
			entry.ProxyTimeout = strings.TrimSpace(strings.TrimPrefix(trimmed, "proxy_timeout "))
		case strings.HasPrefix(trimmed, "proxy_connect_timeout "):
			entry.ConnectTimeout = strings.TrimSpace(strings.TrimPrefix(trimmed, "proxy_connect_timeout "))
		case trimmed == "tcp_nodelay on":
			entry.TCPNodelay = true
		case trimmed == "tcp_nodelay off":
			entry.TCPNodelay = false
		case trimmed == "so_keepalive on" || trimmed == "so_keepalive":
			entry.TCPKeepalive = true
		}
	}

	if entry.ProxyPass == "" {
		return nil, fmt.Errorf("server block without proxy_pass")
	}

	return &StreamEntry{
		Type:    BlockServer,
		ProxyID: proxyID,
		Server:  entry,
	}, nil
}

func (r *ParseResult) ToProxies() []model.Proxy {
	upstreamByID := make(map[string]*UpstreamEntry)
	serverByID := make(map[string]*ServerEntry)

	for _, e := range r.Entries {
		switch e.Type {
		case BlockUpstream:
			if e.ProxyID != "" {
				upstreamByID[e.ProxyID] = e.Upstream
			}
		case BlockServer:
			if e.ProxyID != "" {
				serverByID[e.ProxyID] = e.Server
			}
		}
	}

	// Build upstream name → upstream mapping
	upstreamByName := make(map[string]*UpstreamEntry)
	for _, up := range upstreamByID {
		upstreamByName[up.Name] = up
	}

	// Align upstream IDs with server IDs so they share the same proxy ID
	for _, e := range r.Entries {
		if e.Type != BlockServer || e.Server == nil {
			continue
		}
		if up, ok := upstreamByName[e.Server.ProxyPass]; ok {
			// Find the upstream entry and set its ProxyID to match the server entry
			for j := range r.Entries {
				if r.Entries[j].Type == BlockUpstream && r.Entries[j].Upstream == up {
					r.Entries[j].ProxyID = e.ProxyID
					break
				}
			}
		}
	}

	// Rebuild upstreamByID after alignment
	upstreamByID = make(map[string]*UpstreamEntry)
	for _, e := range r.Entries {
		if e.Type == BlockUpstream && e.ProxyID != "" {
			upstreamByID[e.ProxyID] = e.Upstream
		}
	}

	var proxies []model.Proxy
	seen := make(map[string]bool)
	for _, e := range r.Entries {
		if e.Type != BlockServer || e.Server == nil {
			continue
		}
		pid := e.ProxyID
		if seen[pid] {
			continue
		}
		seen[pid] = true
		srv := e.Server
		proxy := model.Proxy{
			ID:             pid,
			Name:           srv.ProxyPass,
			Listen:         srv.Listen,
			Protocol:       "tcp",
			ProxyProtocol:  srv.ProxyProtocol,
			ProxyTimeout:   srv.ProxyTimeout,
			ConnectTimeout: srv.ConnectTimeout,
			TCPKeepalive:   srv.TCPKeepalive,
			TCPNodelay:     srv.TCPNodelay,
		}
		if srv.IsUDP {
			proxy.Protocol = "udp"
		}

		if up, ok := upstreamByID[pid]; ok {
			proxy.Name = up.Name
			proxy.Backends = up.Backends
			proxy.BalanceMethod = up.BalanceMethod
			proxy.HashConsistent = up.HashConsistent
		}

		proxies = append(proxies, proxy)
	}
	return proxies
}

func sanitizeUpstreamName(name string) string {
	var result strings.Builder
	for _, ch := range strings.ToLower(name) {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '_' {
			result.WriteRune(ch)
		} else if ch == '-' || ch == ' ' || ch == '.' {
			result.WriteRune('_')
		}
	}
	s := result.String()
	s = strings.Trim(s, "_")
	if s == "" {
		s = fmt.Sprintf("upstream_%d", mustRandomInt(9999))
	}
	return "upstream_" + s
}

func mustRandomInt(max int64) int64 {
	n, err := rand.Int(rand.Reader, big.NewInt(max))
	if err != nil {
		return 0
	}
	return n.Int64()
}
