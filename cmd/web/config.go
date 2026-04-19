package main

import (
	"errors"
	"flag"
	"os"
	"strings"
	"time"

	koanjf "github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/providers/confmap"
	koanfenv "github.com/knadh/koanf/providers/env/v2"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"
)

// Config holds all runtime configuration for the web server.
type Config struct {
	HTTP       HTTPConfig       `koanf:"http"`
	DB         DBConfig         `koanf:"db"`
	RateLimits RateLimitsConfig `koanf:"rate_limits"`
}

// HTTPConfig holds HTTP-layer settings.
type HTTPConfig struct {
	// Port is the TCP port the server listens on.
	Port int `koanf:"port"`
	// TrustProxy enables X-Forwarded-For header parsing for rate limiting.
	// Only enable when the server sits behind a trusted reverse proxy that
	// strips client-supplied X-Forwarded-For before appending its own value.
	TrustProxy bool `koanf:"trust_proxy"`
}

// DBConfig holds database connection settings.
type DBConfig struct {
	// URL is the PostgreSQL DSN. Empty means in-memory mode.
	URL string `koanf:"url"`
}

// RateLimitsConfig holds token-bucket parameters for each rate limiter.
// All RPS values are tokens per second; zero means "use compiled default".
type RateLimitsConfig struct {
	IPConnRPS      float64 `koanf:"ip_conn_rps"`
	IPConnBurst    int     `koanf:"ip_conn_burst"`
	IPAuthRPS      float64 `koanf:"ip_auth_rps"`
	IPAuthBurst    int     `koanf:"ip_auth_burst"`
	PlayerActRPS   float64 `koanf:"player_act_rps"`
	PlayerActBurst int     `koanf:"player_act_burst"`
	MsgRPS         float64 `koanf:"msg_rps"`
	MsgBurst       int     `koanf:"msg_burst"`
	// IdleTTL is how long a bucket may be idle before it is evicted.
	// Parsed as a Go duration string (e.g. "10m", "1h").
	IdleTTL time.Duration `koanf:"idle_ttl"`
}

// DefaultConfig returns the compiled-in defaults.
func DefaultConfig() Config {
	return Config{
		HTTP: HTTPConfig{
			Port:       8080,
			TrustProxy: false,
		},
		DB: DBConfig{},
		RateLimits: RateLimitsConfig{
			IPConnRPS:      5,
			IPConnBurst:    10,
			IPAuthRPS:      0.2,
			IPAuthBurst:    5,
			PlayerActRPS:   2,
			PlayerActBurst: 10,
			MsgRPS:         20,
			MsgBurst:       40,
			IdleTTL:        10 * time.Minute,
		},
	}
}

// LoadConfig builds a Config by layering sources in precedence order:
//
//  1. Compiled defaults (DefaultConfig)
//  2. Legacy DATABASE_URL env var (only when GOCHESS__DB__URL is unset)
//  3. JSON config file at --config path (default ./config.json; missing = ok)
//  4. Env vars with prefix GOCHESS_ using __ as the section separator
//     e.g. GOCHESS__HTTP__PORT → http.port
//  5. CLI flags (highest precedence): --port, --db (legacy aliases), --http.port, --db.url
//
// args is typically os.Args[1:] and is accepted as a parameter so tests can
// drive it without touching the real process arguments.
func LoadConfig(args []string) (Config, error) {
	k := koanf.New(".")

	// 1. Compiled defaults via the structs provider.
	if err := k.Load(structs.Provider(DefaultConfig(), "koanf"), nil); err != nil {
		return Config{}, err
	}

	// 2. Legacy DATABASE_URL bridge.
	// Honour it only when the canonical env var is absent so that the newer
	// variable takes precedence when both are set.
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" && os.Getenv("GOCHESS__DB__URL") == "" {
		if err := k.Load(confmap.Provider(map[string]any{"db.url": dbURL}, "."), nil); err != nil {
			return Config{}, err
		}
	}

	// Parse flags first so we know the --config path before loading the file.
	// We register all flags in a single FlagSet and later visit only the ones
	// that were explicitly set (flag.FlagSet.Visit, not VisitAll), then map
	// them to their correct koanf paths manually. This avoids the pitfall of
	// basicflag.Provider mapping "--db" → koanf key "db" (the struct) instead
	// of "db.url".
	fs := flag.NewFlagSet("gochess-web", flag.ContinueOnError)
	fs.Usage = func() {} // suppress auto-print; callers handle usage
	fs.String("config", "config.json", "path to JSON config file")
	fs.Int("port", 0, "HTTP port (legacy alias for --http.port)")
	fs.String("db", "", "PostgreSQL DSN (legacy alias for --db.url)")
	fs.Int("http.port", 0, "HTTP port")
	fs.String("db.url", "", "PostgreSQL DSN")
	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	configPath := fs.Lookup("config").Value.String()

	// 3. JSON config file (missing = ok; malformed = error).
	if err := k.Load(file.Provider(configPath), koanjf.Parser()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return Config{}, err
	}

	// 4. Env vars.
	// Naming convention: GOCHESS__<SECTION>__<KEY> → <section>.<key>
	// Double-underscore is the section separator; single underscores inside
	// field names are preserved (e.g. GOCHESS__RATE_LIMITS__MSG_BURST → rate_limits.msg_burst).
	if err := k.Load(koanfenv.Provider(".", koanfenv.Opt{
		Prefix: "GOCHESS_",
		TransformFunc: func(key, val string) (string, any) {
			// Strip the GOCHESS__ prefix (double underscore marks the boundary
			// between the app prefix and the first section name). Then replace
			// remaining __ section separators with koanf's . delimiter.
			// Example: GOCHESS__RATE_LIMITS__MSG_BURST → rate_limits.msg_burst
			trimmed := strings.TrimPrefix(key, "GOCHESS__")
			path := strings.ToLower(strings.ReplaceAll(trimmed, "__", "."))
			return path, val
		},
	}), nil); err != nil {
		return Config{}, err
	}

	// 5. CLI flags (highest precedence).
	// Walk only the flags that were explicitly set on the command line.
	// Legacy --port and --db are remapped to their proper koanf paths;
	// --http.port and --db.url map directly; --config is skipped (not a koanf key).
	var flagErr error
	fs.Visit(func(f *flag.Flag) {
		if flagErr != nil {
			return
		}
		var kPath string
		switch f.Name {
		case "port":
			kPath = "http.port"
		case "db":
			kPath = "db.url"
		case "config":
			return // consumed above; not a koanf key
		default:
			kPath = f.Name
		}
		if err := k.Set(kPath, f.Value.String()); err != nil {
			flagErr = err
		}
	})
	if flagErr != nil {
		return Config{}, flagErr
	}

	var cfg Config
	if err := k.UnmarshalWithConf("", &cfg, koanf.UnmarshalConf{Tag: "koanf"}); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
