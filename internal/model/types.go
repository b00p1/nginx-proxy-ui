package model

type Backend struct {
	Address     string `json:"address"`
	Weight      int    `json:"weight,omitempty"`
	MaxFails    int    `json:"max_fails,omitempty"`
	FailTimeout string `json:"fail_timeout,omitempty"`
	Backup      bool   `json:"backup,omitempty"`
	SlowStart   string `json:"slow_start,omitempty"`
	MaxConns    int    `json:"max_conns,omitempty"`
}

type Proxy struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Listen   string    `json:"listen"`
	Protocol string    `json:"protocol"`
	Backends []Backend `json:"backends"`

	ProxyProtocol   bool   `json:"proxy_protocol"`
	ProxyTimeout   string `json:"proxy_timeout,omitempty"`
	ConnectTimeout string `json:"connect_timeout,omitempty"`
	TCPKeepalive   bool   `json:"tcp_keepalive"`
	TCPNodelay     bool   `json:"tcp_nodelay"`

	BalanceMethod  string   `json:"balance_method,omitempty"`
	HashConsistent bool     `json:"hash_consistent,omitempty"`
	Extra          []string `json:"extra,omitempty"`
}

type ConfigStatus struct {
	Valid      bool   `json:"valid"`
	LastReload string `json:"last_reload"`
	LastTest   string `json:"last_test"`
	TestOutput string `json:"test_output"`
}
