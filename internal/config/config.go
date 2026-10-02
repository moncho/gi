package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/rcarmo/gi/internal/skills"
)

type RuntimeConfig struct {
	WorkspaceRoot        string             `json:"workspace_root"`
	AssistantName        string             `json:"assistant_name"`
	AssistantAvatar      string             `json:"assistant_avatar"`
	UserName             string             `json:"user_name"`
	UserAvatar           string             `json:"user_avatar"`
	UserAvatarBackground string             `json:"user_avatar_background"`
	DefaultProvider      string             `json:"default_provider"`
	DefaultModel         string             `json:"default_model"`
	DefaultThinkingLevel string             `json:"default_thinking_level"`
	EnabledModels        []string           `json:"enabled_models"`
	Agents               AgentsConfig       `json:"agents"`
	Session              SessionConfig      `json:"session"`
	Routing              ModelRoutingConfig `json:"routing"`
	MaxIterations        int                `json:"max_iterations"`
	ScrollbackLimit      int                `json:"scrollback_limit"`
	TUIHistoryLimit      int                `json:"tui_history_limit"`
	TUIClipboardMode     string             `json:"tui_clipboard_mode"`
	// EnabledModelsConfigured is false when EnabledModels is gi's built-in
	// fallback; Pi then has no "scoped" model list.
	EnabledModelsConfigured bool `json:"-"`
	// TUIWheelScrollLines is Pi's fullscreenWheelScrollLines; 0 means "auto".
	TUIWheelScrollLines int `json:"tui_wheel_scroll_lines"`
	// Theme is Pi's theme setting (project .pi/settings.json, else global).
	Theme string `json:"theme,omitempty"`
	// TUIMode is Pi's tuiMode setting (project, else global); the -tui-mode
	// flag overrides it.
	TUIMode string `json:"tui_mode,omitempty"`
	// QuietStartup is Pi's quietStartup: "" (false: header and loaded
	// resources), "true" (neither) or "header" (header only).
	QuietStartup string `json:"quiet_startup,omitempty"`
	// DefaultToolsLayers are Pi's defaultTools lists, user then project.
	DefaultToolsLayers [][]string `json:"default_tools_layers,omitempty"`
	// ExtensionsLayers are Pi's extensions lists, user then project.
	ExtensionsLayers [][]string `json:"extensions_layers,omitempty"`
	// Codemode is Pi's codemode settings (project values override user ones).
	Codemode       CodemodeSettings       `json:"codemode"`
	Compaction     CompactionSettings     `json:"compaction"`
	Retry          ProviderRetrySettings  `json:"retry"`
	Hooks          HookSettings           `json:"hooks"`
	Peering        PeeringSettings        `json:"peering"`
	Passkeys       PasskeySettings        `json:"passkeys"`
	InboundWork    InboundWorkSettings    `json:"inbound_work"`
	WorkspaceIndex WorkspaceIndexSettings `json:"workspace_index"`
	SystemPrompt   string                 `json:"-"`
	Discovery      skills.Discovery       `json:"-"`
}

type piclawConfig struct {
	Assistant struct {
		AssistantName   string `json:"assistantName"`
		AssistantAvatar string `json:"assistantAvatar"`
	} `json:"assistant"`
	User struct {
		UserName             string `json:"userName"`
		UserAvatar           string `json:"userAvatar"`
		UserAvatarBackground string `json:"userAvatarBackground"`
	} `json:"user"`
}

type CompactionSettings struct {
	Enabled          bool   `json:"enabled"`
	ContextWindow    int    `json:"context_window"`
	ReserveTokens    int    `json:"reserve_tokens"`
	KeepRecentTokens int    `json:"keep_recent_tokens"`
	ThresholdTokens  int    `json:"threshold_tokens"`
	Strategy         string `json:"strategy"`
}

type HookSettings struct {
	TimeoutMS int    `json:"timeout_ms"`
	OnError   string `json:"on_error"`
	OnTimeout string `json:"on_timeout"`
}

type PasskeySettings struct {
	RPID    string   `json:"rp_id"`
	Origins []string `json:"origins"`
}

type PeeringSettings struct {
	Enabled         bool   `json:"enabled"`
	Hostname        string `json:"hostname"`
	StateDir        string `json:"state_dir"`
	AuthKeyEnv      string `json:"auth_key_env"`
	AuthKeyKeychain string `json:"auth_key_keychain"`
}

type InboundWorkSettings struct {
	Enabled    bool   `json:"enabled"`
	IntervalMS int    `json:"interval_ms"`
	BatchSize  int    `json:"batch_size"`
	WorkerID   string `json:"worker_id"`
	LeaseTTLMS int    `json:"lease_ttl_ms"`
}

// WorkspaceIndexSettings is startup-only. Extra roots are required unless
// listed explicitly as optional; the built-in notes and .pi/skills roots are
// always optional. Validation is performed when resolving an index scope.
type WorkspaceIndexSettings struct {
	ExtraRoots      []string `json:"extraRoots"`
	ExtraExtensions []string `json:"extraExtensions"`
	OptionalRoots   []string `json:"optionalRoots"`
}

type piSettings struct {
	DefaultTools []string          `json:"defaultTools"`
	Extensions   []string          `json:"extensions"`
	Codemode     *CodemodeSettings `json:"codemode"`
	// Theme is Pi's theme setting: a theme name ("dark", "light", custom) or
	// an auto pair "light/dark" resolved by the terminal's detected scheme.
	Theme                string   `json:"theme"`
	// TUIMode is Pi's tuiMode: "fullscreen" (default) or "regular".
	TUIMode string `json:"tuiMode"`
	// QuietStartup is Pi's quietStartup: false, true or "header".
	QuietStartup json.RawMessage `json:"quietStartup"`
	DefaultProvider      string   `json:"defaultProvider"`
	DefaultModel         string   `json:"defaultModel"`
	DefaultThinkingLevel string   `json:"defaultThinkingLevel"`
	EnabledModels        []string `json:"enabledModels"`
	MaxIterations        int      `json:"maxIterations"`
	TUIScrollbackLimit   int      `json:"tuiScrollbackLimit"`
	TUIHistoryLimit      int      `json:"tuiHistoryLimit"`
	TUIClipboardMode     string   `json:"tuiClipboardMode"`
	// Pi's fullscreenWheelScrollLines: a number of lines, or "auto".
	FullscreenWheelScrollLines any                    `json:"fullscreenWheelScrollLines"`
	Compaction                 CompactionSettings     `json:"compaction"`
	Retry                      ProviderRetrySettings  `json:"retry"`
	Hooks                      HookSettings           `json:"hooks"`
	Peering                    PeeringSettings        `json:"peering"`
	Passkeys                   PasskeySettings        `json:"passkeys"`
	InboundWork                *InboundWorkSettings   `json:"inboundWork"`
	WorkspaceIndex             WorkspaceIndexSettings `json:"workspaceIndex"`
	Agents                     AgentsConfig           `json:"agents"`
	Session                    SessionConfig          `json:"session"`
	Routing                    ModelRoutingConfig     `json:"routing"`
}

func Load(workspaceRoot string) RuntimeConfig {
	workspaceRoot = strings.TrimSpace(workspaceRoot)
	if workspaceRoot == "" {
		workspaceRoot = DefaultWorkspaceRoot()
	}
	cfg := RuntimeConfig{WorkspaceRoot: workspaceRoot, Compaction: CompactionSettings{Enabled: true}, InboundWork: InboundWorkSettings{Enabled: true}}
	var projectTools, projectExtensions []string
	var projectCodemode *CodemodeSettings
	var pc piclawConfig
	if err := readJSON(filepath.Join(workspaceRoot, ".piclaw", "config.json"), &pc); err == nil {
		cfg.AssistantName = pc.Assistant.AssistantName
		cfg.AssistantAvatar = pc.Assistant.AssistantAvatar
		cfg.UserName = pc.User.UserName
		cfg.UserAvatar = pc.User.UserAvatar
		cfg.UserAvatarBackground = pc.User.UserAvatarBackground
	}
	var ps piSettings
	if err := readJSON(filepath.Join(workspaceRoot, ".pi", "settings.json"), &ps); err == nil {
		cfg.DefaultProvider = ps.DefaultProvider
		cfg.DefaultModel = ps.DefaultModel
		cfg.DefaultThinkingLevel = ps.DefaultThinkingLevel
		cfg.EnabledModels = append([]string(nil), ps.EnabledModels...)
		cfg.MaxIterations = ps.MaxIterations
		cfg.ScrollbackLimit = ps.TUIScrollbackLimit
		cfg.TUIHistoryLimit = ps.TUIHistoryLimit
		cfg.TUIClipboardMode = normalizeClipboardMode(ps.TUIClipboardMode)
		cfg.TUIWheelScrollLines = wheelScrollLines(ps.FullscreenWheelScrollLines)
		cfg.Compaction = ps.Compaction
		cfg.Retry = ps.Retry
		cfg.Hooks = ps.Hooks
		cfg.Peering = ps.Peering
		cfg.Passkeys = ps.Passkeys
		if ps.InboundWork != nil {
			cfg.InboundWork = *ps.InboundWork
		}
		cfg.Agents = ps.Agents
		cfg.Session = ps.Session
		cfg.Routing = ps.Routing
		cfg.WorkspaceIndex = ps.WorkspaceIndex
		cfg.Theme = strings.TrimSpace(ps.Theme)
		cfg.TUIMode = strings.TrimSpace(ps.TUIMode)
		cfg.QuietStartup = parseQuietStartup(ps.QuietStartup)
		projectTools, projectExtensions, projectCodemode = ps.DefaultTools, ps.Extensions, ps.Codemode
	}
	applyGlobalPiSettings(&cfg)
	// Project settings apply on top of the user's (Pi).
	if projectTools != nil {
		cfg.DefaultToolsLayers = append(cfg.DefaultToolsLayers, projectTools)
	}
	if projectExtensions != nil {
		cfg.ExtensionsLayers = append(cfg.ExtensionsLayers, projectExtensions)
	}
	if projectCodemode != nil {
		if projectCodemode.Mode != "" {
			cfg.Codemode.Mode = projectCodemode.Mode
		}
		if projectCodemode.InlineBudget != nil {
			cfg.Codemode.InlineBudget = projectCodemode.InlineBudget
		}
	}
	if discovery, err := skills.Discover(workspaceRoot); err == nil {
		cfg.Discovery = discovery
	}
	if cfg.AssistantName == "" {
		cfg.AssistantName = "Gi"
	}
	if cfg.UserName == "" {
		cfg.UserName = "User"
	}
	if strings.TrimSpace(cfg.DefaultProvider) == "" {
		cfg.DefaultProvider = "opencode-zen"
	}
	cfg.EnabledModelsConfigured = len(cfg.EnabledModels) > 0
	if len(cfg.EnabledModels) == 0 {
		cfg.EnabledModels = []string{"opencode-zen/minimax-m2.5-free"}
	}
	if cfg.DefaultModel == "" && len(cfg.EnabledModels) > 0 {
		cfg.DefaultModel = cfg.EnabledModels[0]
	}
	if strings.TrimSpace(cfg.DefaultThinkingLevel) == "" {
		// Pi's DEFAULT_THINKING_LEVEL.
		cfg.DefaultThinkingLevel = "medium"
	}
	if len(cfg.Session.Dimensions) == 0 {
		cfg.Session.Dimensions = []string{"chat"}
	}
	if cfg.MaxIterations <= 0 {
		cfg.MaxIterations = 64
	}
	if cfg.ScrollbackLimit <= 0 {
		cfg.ScrollbackLimit = 1000
	}
	if cfg.TUIHistoryLimit <= 0 {
		cfg.TUIHistoryLimit = 10000
	}
	cfg.TUIClipboardMode = normalizeClipboardMode(cfg.TUIClipboardMode)
	applyCompactionDefaults(&cfg.Compaction)
	applyHookDefaults(&cfg.Hooks)
	applyInboundWorkDefaults(&cfg.InboundWork)
	if len(cfg.Agents.List) == 0 {
		cfg.Agents.List = []AgentConfig{{ID: "agent", Name: cfg.AssistantName, Default: true, Model: cfg.DefaultModel}}
	}
	// Load workspace instructions from AGENTS.md and wrap them in gi's runtime prompt.
	workspaceInstructions := ""
	agentsPath := filepath.Join(workspaceRoot, "AGENTS.md")
	if data, err := os.ReadFile(agentsPath); err == nil && len(data) > 0 {
		workspaceInstructions = string(data)
	}
	cfg.SystemPrompt = buildSystemPrompt(cfg, workspaceInstructions)
	return cfg
}

func PersistModelSelection(workspaceRoot, provider, model, thinking string, enabledModels []string) error {
	fields := map[string]any{}
	if strings.TrimSpace(provider) != "" {
		fields["defaultProvider"] = strings.TrimSpace(provider)
	}
	if strings.TrimSpace(model) != "" {
		fields["defaultModel"] = strings.TrimSpace(model)
	}
	if strings.TrimSpace(thinking) != "" {
		fields["defaultThinkingLevel"] = strings.TrimSpace(thinking)
	}
	models := append([]string(nil), enabledModels...)
	if strings.TrimSpace(model) != "" && !contains(models, model) {
		models = append(models, model)
	}
	if len(models) > 0 {
		fields["enabledModels"] = models
	}
	return persistPiFields(workspaceRoot, fields)
}

func PersistClipboardMode(workspaceRoot, mode string) error {
	return persistPiFields(workspaceRoot, map[string]any{"tuiClipboardMode": normalizeClipboardMode(mode)})
}

func normalizeClipboardMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	// Preserve unset so interactive selection and /copy can have different defaults.
	case "", "osc52", "native", "auto":
		return strings.ToLower(strings.TrimSpace(mode))
	default:
		return "off"
	}
}

func PersistScrollbackLimit(workspaceRoot string, limit int) error {
	if limit <= 0 {
		return errors.New("scrollback limit must be > 0")
	}
	return persistPiFields(workspaceRoot, map[string]any{"tuiScrollbackLimit": limit})
}

func PersistTUIHistoryLimit(workspaceRoot string, limit int) error {
	if limit <= 0 {
		return errors.New("history limit must be > 0")
	}
	return persistPiFields(workspaceRoot, map[string]any{"tuiHistoryLimit": limit})
}

func applyInboundWorkDefaults(settings *InboundWorkSettings) {
	if settings.IntervalMS <= 0 {
		settings.IntervalMS = 500
	}
	if settings.BatchSize <= 0 {
		settings.BatchSize = 8
	}
	if strings.TrimSpace(settings.WorkerID) == "" {
		settings.WorkerID = "web-runtime"
	}
	if settings.LeaseTTLMS <= 0 {
		settings.LeaseTTLMS = 2000
	}
}

func contains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func readJSON(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return errors.New("empty file")
	}
	return json.Unmarshal(data, target)
}

// piAgentDir is Pi's global config directory (PI_CODING_AGENT_DIR or
// ~/.pi/agent).
func piAgentDir() string {
	if dir := strings.TrimSpace(os.Getenv("PI_CODING_AGENT_DIR")); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return ""
	}
	return filepath.Join(home, ".pi", "agent")
}

// applyGlobalPiSettings merges Pi's global settings.json under the project
// settings, as Pi does: project values win, global values fill the model,
// provider, thinking level and scoped models when the project leaves them unset.
func applyGlobalPiSettings(cfg *RuntimeConfig) {
	dir := piAgentDir()
	if dir == "" {
		return
	}
	var global piSettings
	if err := readJSON(filepath.Join(dir, "settings.json"), &global); err != nil {
		return
	}
	if strings.TrimSpace(cfg.DefaultProvider) == "" && strings.TrimSpace(cfg.DefaultModel) == "" {
		cfg.DefaultProvider = global.DefaultProvider
		cfg.DefaultModel = global.DefaultModel
	}
	if strings.TrimSpace(cfg.DefaultThinkingLevel) == "" {
		cfg.DefaultThinkingLevel = global.DefaultThinkingLevel
	}
	if cfg.TUIWheelScrollLines == 0 {
		cfg.TUIWheelScrollLines = wheelScrollLines(global.FullscreenWheelScrollLines)
	}
	if cfg.Theme == "" {
		cfg.Theme = strings.TrimSpace(global.Theme)
	}
	if cfg.TUIMode == "" {
		cfg.TUIMode = strings.TrimSpace(global.TUIMode)
	}
	if cfg.QuietStartup == "" {
		cfg.QuietStartup = parseQuietStartup(global.QuietStartup)
	}
	if global.DefaultTools != nil {
		cfg.DefaultToolsLayers = append([][]string{global.DefaultTools}, cfg.DefaultToolsLayers...)
	}
	if global.Extensions != nil {
		cfg.ExtensionsLayers = append([][]string{global.Extensions}, cfg.ExtensionsLayers...)
	}
	if global.Codemode != nil {
		cfg.Codemode = *global.Codemode
	}
	if len(cfg.EnabledModels) == 0 && len(global.EnabledModels) > 0 {
		cfg.EnabledModels = append([]string(nil), global.EnabledModels...)
	}
}

// wheelScrollLines normalises Pi's fullscreenWheelScrollLines: a finite
// number is clamped to 1..100; anything else ("auto", unset) is 0 (auto).
func wheelScrollLines(v any) int {
	n, ok := v.(float64)
	if !ok || n != n {
		return 0
	}
	return max(1, min(100, int(n)))
}

// CodemodeSettings are Pi's codemode settings.
type CodemodeSettings struct {
	Mode         string `json:"mode,omitempty"`         // "on" (default) or "only"
	InlineBudget *int   `json:"inlineBudget,omitempty"` // description token budget
}

// ToolEnabledByDefault applies Pi's defaultTools layers to one tool: a list
// with plain names replaces the selection, +name/-name edit it.
func (c RuntimeConfig) ToolEnabledByDefault(name string, initial bool) bool {
	enabled := initial
	for _, layer := range c.DefaultToolsLayers {
		plain := false
		for _, entry := range layer {
			if e := strings.TrimSpace(entry); e != "" && !strings.HasPrefix(e, "+") && !strings.HasPrefix(e, "-") {
				plain = true
			}
		}
		if plain {
			enabled = false
		}
		for _, entry := range layer {
			switch e := strings.TrimSpace(entry); {
			case e == name, e == "+"+name:
				enabled = true
			case e == "-"+name:
				enabled = false
			}
		}
	}
	return enabled
}

// BuiltinDisabled reports whether "-builtin:<name>" appears in Pi's
// extensions setting (the last layer to mention it wins).
func (c RuntimeConfig) BuiltinDisabled(name string) bool {
	disabled := false
	for _, layer := range c.ExtensionsLayers {
		for _, entry := range layer {
			switch strings.TrimSpace(entry) {
			case "-builtin:" + name:
				disabled = true
			case "+builtin:" + name, "builtin:" + name:
				disabled = false
			}
		}
	}
	return disabled
}

// parseQuietStartup reads Pi's quietStartup (true, false or "header"):
// "true", "header" or "" for false and anything else.
func parseQuietStartup(raw json.RawMessage) string {
	switch strings.TrimSpace(string(raw)) {
	case "true":
		return "true"
	case `"header"`:
		return "header"
	}
	return ""
}
