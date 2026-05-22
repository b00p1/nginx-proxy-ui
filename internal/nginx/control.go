package nginx

import (
	"bytes"
	"fmt"
	"os/exec"
	"sync"
	"time"
)

type Control struct {
	mu         sync.Mutex
	lastReload string
	lastTest   string
	testOutput string
}

func New() *Control {
	return &Control{}
}

func (c *Control) TestConfig() (bool, string, error) {
	cmd := exec.Command("nginx", "-t")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()

	output := out.String()
	now := time.Now().Format(time.RFC3339)

	c.mu.Lock()
	c.lastTest = now
	c.testOutput = output
	c.mu.Unlock()

	if err != nil {
		return false, output, nil
	}
	return true, output, nil
}

func (c *Control) Reload() (string, error) {
	// First test the config
	valid, output, err := c.TestConfig()
	if err != nil {
		return output, fmt.Errorf("test config failed: %w", err)
	}
	if !valid {
		return output, fmt.Errorf("config is invalid")
	}

	cmd := exec.Command("nginx", "-s", "reload")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err = cmd.Run()

	now := time.Now().Format(time.RFC3339)
	c.mu.Lock()
	c.lastReload = now
	c.mu.Unlock()

	if err != nil {
		return out.String(), fmt.Errorf("reload failed: %w", err)
	}
	return out.String(), nil
}

func (c *Control) Status() (string, string, string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastReload, c.lastTest, c.testOutput
}
