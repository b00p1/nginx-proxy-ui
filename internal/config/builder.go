package config

import (
	"fmt"
	"strings"

	"nginx-proxy-manager/internal/model"
)

func BuildStreamContent(entries []StreamEntry, proxies []model.Proxy) string {
	proxyByID := make(map[string]model.Proxy)
	for _, p := range proxies {
		proxyByID[p.ID] = p
	}

	upstreamNameByID := make(map[string]string)
	for _, p := range proxies {
		name := sanitizeUpstreamName(p.Name)
		upstreamNameByID[p.ID] = name
	}

	var parts []string
	existingUpstream := make(map[string]bool)

	for _, entry := range entries {
		switch entry.Type {
		case BlockRaw:
			if entry.Raw != "" {
				parts = append(parts, entry.Raw)
			}
		case BlockUpstream:
			if proxy, ok := proxyByID[entry.ProxyID]; ok {
				newName := upstreamNameByID[entry.ProxyID]
				if !existingUpstream[newName] {
					parts = append(parts, buildUpstreamBlock(proxy, newName))
					existingUpstream[newName] = true
				}
			}
		case BlockServer:
			if proxy, ok := proxyByID[entry.ProxyID]; ok {
				newName := upstreamNameByID[entry.ProxyID]
				parts = append(parts, buildServerBlock(proxy, newName))
			}
		}
	}

	// Append new proxies (not in existing entries)
	seen := make(map[string]bool)
	for _, e := range entries {
		if e.ProxyID != "" {
			seen[e.ProxyID] = true
		}
	}
	for _, p := range proxies {
		if seen[p.ID] {
			continue
		}
		name := upstreamNameByID[p.ID]
		if !existingUpstream[name] && len(p.Backends) > 0 {
			parts = append(parts, buildUpstreamBlock(p, name))
			existingUpstream[name] = true
		}
		parts = append(parts, buildServerBlock(p, name))
	}

	return strings.Join(parts, "\n\n")
}

func buildUpstreamBlock(proxy model.Proxy, name string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# nginx-proxy-manager-id: %s\nupstream %s {\n", proxy.ID, name)
	for _, backend := range proxy.Backends {
		b.WriteString("    server ")
		b.WriteString(backend.Address)
		if backend.Weight > 0 {
			fmt.Fprintf(&b, " weight=%d", backend.Weight)
		}
		if backend.MaxFails > 0 {
			fmt.Fprintf(&b, " max_fails=%d", backend.MaxFails)
		}
		if backend.FailTimeout != "" {
			fmt.Fprintf(&b, " fail_timeout=%s", backend.FailTimeout)
		}
		if backend.Backup {
			b.WriteString(" backup")
		}
		if backend.SlowStart != "" {
			fmt.Fprintf(&b, " slow_start=%s", backend.SlowStart)
		}
		if backend.MaxConns > 0 {
			fmt.Fprintf(&b, " max_conns=%d", backend.MaxConns)
		}
		b.WriteString(";\n")
	}
	switch proxy.BalanceMethod {
	case "least_conn":
		b.WriteString("    least_conn;\n")
	case "random":
		b.WriteString("    random;\n")
	case "hash":
		if proxy.HashConsistent {
			b.WriteString("    hash $remote_addr consistent;\n")
		} else {
			b.WriteString("    hash $remote_addr;\n")
		}
	}
	b.WriteString("}")
	return b.String()
}

func buildServerBlock(proxy model.Proxy, upstreamName string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# nginx-proxy-manager-id: %s\nserver {\n", proxy.ID)
	if proxy.Protocol == "udp" {
		fmt.Fprintf(&b, "    listen %s udp;\n", proxy.Listen)
	} else {
		fmt.Fprintf(&b, "    listen %s;\n", proxy.Listen)
	}
	fmt.Fprintf(&b, "    proxy_pass %s;\n", upstreamName)
	if proxy.ProxyProtocol {
		b.WriteString("    proxy_protocol on;\n")
	}
	if proxy.ProxyTimeout != "" {
		fmt.Fprintf(&b, "    proxy_timeout %s;\n", proxy.ProxyTimeout)
	}
	if proxy.ConnectTimeout != "" {
		fmt.Fprintf(&b, "    proxy_connect_timeout %s;\n", proxy.ConnectTimeout)
	}
	if proxy.TCPNodelay {
		b.WriteString("    tcp_nodelay on;\n")
	}
	if proxy.TCPKeepalive {
		b.WriteString("    so_keepalive on;\n")
	}
	for _, extra := range proxy.Extra {
		extra = strings.TrimSpace(extra)
		if extra != "" && !strings.HasSuffix(extra, ";") {
			extra += ";"
		}
		if extra != "" {
			fmt.Fprintf(&b, "    %s\n", extra)
		}
	}
	b.WriteString("}")
	return b.String()
}
