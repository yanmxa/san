package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"go.uber.org/zap"

	sdkmcp "github.com/genai-io/sdk-go/pkg/agent/mcp"
	"github.com/genai-io/sdk-go/pkg/ai"

	"github.com/genai-io/san/internal/core"
	glog "github.com/genai-io/san/internal/log"
)

// One MCP server, and San's business with it.
//
// The protocol is not here. Reaching a server, speaking JSON-RPC to it over a
// pipe or an HTTP stream, and turning what it advertises into something the
// loop can call are sdk-go's — pkg/agent/mcp hands back core.Tool values, the
// same type the rest of San's tools already are.
//
// What is here is what the SDK has no opinion about: when a connection is made
// and dropped, and what the /mcp listing shows.

// conn is one live session, as this package uses it.
//
// The real one is the SDK's. The registry's tests supply their own, because
// what they are about is leases and epochs and replacement — standing a server
// up per case would test the SDK's transport over and over instead, and it
// already has tests for that.
type conn interface {
	Tools(ctx context.Context) ([]core.Tool, error)
	Resources(ctx context.Context) ([]sdkmcp.Resource, error)
	Prompts(ctx context.Context) ([]sdkmcp.Prompt, error)
	Alive() bool
	Close() error
}

// Client is one MCP server as San holds it: connected or not, with what it
// last said it offers.
type Client struct {
	config ServerConfig

	// dial opens the session. Tests replace it; see conn.
	dial func(ctx context.Context, onToolsChanged func()) (conn, error)

	mu        sync.RWMutex
	session   conn
	tools     []core.Tool
	resources []MCPResource
	prompts   []MCPPrompt

	onToolsChanged func()
}

// NewClient returns a client for one configured server. Nothing is reached
// until Connect.
func NewClient(config ServerConfig) *Client {
	return &Client{config: config, dial: dialSDK(config)}
}

// dialSDK opens the real session for this configuration.
//
// The server's own tool names come back unqualified: San assembles
// mcp__server__tool in the registry, where it has since before this package
// spoke to the SDK, and where its permission rules and transcripts match on it.
func dialSDK(config ServerConfig) func(context.Context, func()) (conn, error) {
	return func(ctx context.Context, onToolsChanged func()) (conn, error) {
		server := sdkmcp.Server{
			Command: config.Command,
			Args:    config.Args,
			Env:     config.Env,
			URL:     config.URL,
			Headers: config.Headers,
			SSE:     config.GetType() == TransportSSE,
			// A server that fails to start usually says why on its stderr and
			// nowhere else. San is full-screen, so it cannot go to the
			// terminal: it goes to the log the user already has.
			Stderr: serverLog{name: config.Name},
		}
		var opts []sdkmcp.Option
		if onToolsChanged != nil {
			opts = append(opts, sdkmcp.OnToolsChanged(onToolsChanged))
		}
		c, err := sdkmcp.Connect(ctx, server, opts...)
		if err != nil {
			return nil, err
		}
		return sdkSession{c}, nil
	}
}

// serverLog is one server's stderr, as a line in San's log. A server that
// cannot start says why there, and a full-screen program has nowhere else to
// put it — writing to the terminal paints over the interface.
type serverLog struct{ name string }

func (l serverLog) Write(p []byte) (int, error) {
	if line := strings.TrimRight(string(p), "\n"); line != "" {
		glog.Logger().Debug("mcp server output", zap.String("server", l.name), zap.String("line", line))
	}
	return len(p), nil
}

// sdkSession is the SDK's client under the three questions this package asks.
type sdkSession struct{ *sdkmcp.Client }

func (s sdkSession) Tools(ctx context.Context) ([]core.Tool, error) {
	return s.Client.Tools(ctx)
}

// Connect opens the session and reads what the server offers. Connecting a
// client that is already connected is a no-op rather than a second process.
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	if c.session != nil && c.session.Alive() {
		c.mu.Unlock()
		return nil
	}
	dial := c.dial
	c.mu.Unlock()

	session, err := dial(ctx, c.notifyToolsChanged)
	if err != nil {
		return err
	}

	c.mu.Lock()
	c.session = session
	c.mu.Unlock()

	if err := c.refresh(ctx); err != nil {
		_ = c.Disconnect()
		return err
	}
	return nil
}

// refresh re-reads what the server offers. The read happens outside the lock: a
// slow server must not hold up whoever is asking whether this one is connected.
func (c *Client) refresh(ctx context.Context) error {
	session := c.conn()
	if session == nil {
		return fmt.Errorf("not connected")
	}
	tools, err := session.Tools(ctx)
	if err != nil {
		return err
	}
	// A server need not offer resources or prompts, and saying so is not a
	// failure worth dropping the connection over.
	resources, _ := session.Resources(ctx)
	prompts, _ := session.Prompts(ctx)

	c.mu.Lock()
	c.tools = tools
	c.resources = toMCPResources(resources)
	c.prompts = toMCPPrompts(prompts)
	c.mu.Unlock()
	return nil
}

// Disconnect ends the session. Disconnecting a client that is not connected is
// a no-op.
func (c *Client) Disconnect() error {
	c.mu.Lock()
	session := c.session
	c.session, c.tools, c.resources, c.prompts = nil, nil, nil, nil
	c.mu.Unlock()

	if session == nil {
		return nil
	}
	return session.Close()
}

// IsConnected reports whether the session is up. A session whose server has
// died says no, which is what tells the registry to open a new one.
func (c *Client) IsConnected() bool {
	session := c.conn()
	return session != nil && session.Alive()
}

func (c *Client) conn() conn {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.session
}

// ListTools re-reads what the server offers.
func (c *Client) ListTools(ctx context.Context) ([]MCPTool, error) {
	if err := c.refresh(ctx); err != nil {
		return nil, err
	}
	return c.GetCachedTools(), nil
}

// GetCachedTools is what the server last said it offers, without asking again.
// The /mcp listing and the tool picker are drawn from this, so neither blocks
// on a server.
func (c *Client) GetCachedTools() []MCPTool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	out := make([]MCPTool, 0, len(c.tools))
	for _, t := range c.tools {
		schema := t.Schema()
		tool := MCPTool{Name: schema.Name, Description: schema.Description}
		if raw, err := json.Marshal(schema.Definition); err == nil {
			tool.InputSchema = raw
		}
		out = append(out, tool)
	}
	return out
}

// CallTool runs one of this server's tools, by the name the server calls it.
func (c *Client) CallTool(ctx context.Context, name string, arguments map[string]any) (*ToolResult, error) {
	tool, ok := c.tool(name)
	if !ok {
		if !c.IsConnected() {
			return nil, fmt.Errorf("not connected")
		}
		return nil, fmt.Errorf("MCP server %s does not offer %s", c.config.Name, name)
	}

	input, err := json.Marshal(arguments)
	if err != nil {
		return nil, err
	}

	// A tool that failed is not a failed call: the loop tells the model what
	// the server said so it can correct itself, and IsError is how it knows.
	result, runErr := tool.Run(ctx, ai.ToolCall{Name: name, Input: string(input)})
	out := &ToolResult{Content: toolResultContent(result.Content), IsError: runErr != nil}
	if runErr != nil && len(out.Content) == 0 {
		out.Content = []ToolResultContent{{Type: "text", Text: runErr.Error()}}
	}
	return out, nil
}

func (c *Client) tool(name string) (core.Tool, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, t := range c.tools {
		if t.Schema().Name == name {
			return t, true
		}
	}
	return nil, false
}

// toolResultContent is what the server returned, in the shape San's interface
// draws. A block it has no way to draw is named rather than dropped.
func toolResultContent(content ai.Content) []ToolResultContent {
	out := make([]ToolResultContent, 0, len(content))
	for _, b := range content {
		switch b.Type {
		case ai.BlockText:
			out = append(out, ToolResultContent{Type: "text", Text: b.Text})
		case ai.BlockImage:
			if b.Image != nil {
				out = append(out, ToolResultContent{Type: "image", Data: b.Image.Data, MimeType: b.Image.MediaType})
			}
		default:
			out = append(out, ToolResultContent{Type: "text", Text: fmt.Sprintf("(%s)", b.Type)})
		}
	}
	return out
}

func toMCPResources(in []sdkmcp.Resource) []MCPResource {
	out := make([]MCPResource, 0, len(in))
	for _, r := range in {
		out = append(out, MCPResource{URI: r.URI, Name: r.Name, Description: r.Description, MimeType: r.MediaType})
	}
	return out
}

func toMCPPrompts(in []sdkmcp.Prompt) []MCPPrompt {
	out := make([]MCPPrompt, 0, len(in))
	for _, p := range in {
		out = append(out, MCPPrompt{Name: p.Name, Description: p.Description})
	}
	return out
}

// GetCachedResources is what the server last said it offers to read.
func (c *Client) GetCachedResources() []MCPResource {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]MCPResource, len(c.resources))
	copy(out, c.resources)
	return out
}

// SetOnToolsChanged installs the callback for a server that says its tool list
// has changed. It may be set before or after Connect: the session calls back
// through the client, which reads the field when the notification arrives.
func (c *Client) SetOnToolsChanged(callback func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onToolsChanged = callback
}

// notifyToolsChanged re-reads the server's tools and then tells whoever asked.
// The re-read is the point: a callback that only says "something changed"
// leaves every consumer to ask again, and they would all ask at once.
func (c *Client) notifyToolsChanged() {
	if err := c.refresh(context.Background()); err != nil {
		return
	}
	c.mu.RLock()
	callback := c.onToolsChanged
	c.mu.RUnlock()
	if callback != nil {
		callback()
	}
}

// Config is the configuration this client was built from.
func (c *Client) Config() ServerConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.config
}

// ToServer is this server as the /mcp listing shows it.
func (c *Client) ToServer() Server {
	c.mu.RLock()
	status := c.statusLocked()
	resources := make([]MCPResource, len(c.resources))
	copy(resources, c.resources)
	prompts := make([]MCPPrompt, len(c.prompts))
	copy(prompts, c.prompts)
	c.mu.RUnlock()

	return Server{
		Config:    c.config,
		Status:    status,
		Tools:     c.GetCachedTools(),
		Resources: resources,
		Prompts:   prompts,
	}
}

// statusLocked separates a client that was never connected from one whose
// session has since died. The second is what a user needs to see to know that
// reconnecting is worth trying.
func (c *Client) statusLocked() ServerStatus {
	switch {
	case c.session == nil:
		return StatusDisconnected
	case c.session.Alive():
		return StatusConnected
	}
	return StatusError
}

// MarshalJSON implements json.Marshaler for debugging.
func (c *Client) MarshalJSON() ([]byte, error) { return json.Marshal(c.ToServer()) }
