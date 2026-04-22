package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"mihoctl/internal/core"
)

const (
	defaultLanguage = "zh-CN"
)

type BootstrapOptions struct {
	Lang       string
	ConfigPath string
}

type Paths struct {
	AppHome    string
	ConfigFile string
	StateFile  string
	BinDir     string
	SubDir     string
	LogDir     string
	LogFile    string
	ExecPath   string
	ExecDir    string
	CWD        string
}

type Config struct {
	Language            string         `json:"language"`
	Mode                string         `json:"mode"`
	Core                CoreConfig     `json:"core"`
	Controller          Controller     `json:"controller"`
	Mihomo              Mihomo         `json:"mihomo"`
	SystemProxy         SystemProxy    `json:"system_proxy"`
	Subscriptions       []Subscription `json:"subscriptions"`
	DefaultSubscription string         `json:"default_subscription"`
	HealthCheck         HealthCheck    `json:"health_check"`
}

type CoreConfig struct {
	Repo              string `json:"repo"`
	InstallDir        string `json:"install_dir"`
	DatabaseDir       string `json:"database_dir"`
	AutoCheckUpdates  bool   `json:"auto_check_updates"`
	CheckIntervalHour int    `json:"check_interval_hour"`
}

type Controller struct {
	Address string `json:"address"`
	Secret  string `json:"secret"`
}

type Mihomo struct {
	BinaryPath string `json:"binary_path"`
	ConfigPath string `json:"config_path"`
	WorkDir    string `json:"work_dir"`
}

type SystemProxy struct {
	ServiceName string `json:"service_name"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
}

type Subscription struct {
	Name       string `json:"name"`
	URL        string `json:"url"`
	ConfigPath string `json:"config_path"`
}

type HealthCheck struct {
	URL       string `json:"url"`
	TimeoutMS int    `json:"timeout_ms"`
}

func ParseBootstrapOptions(args []string) BootstrapOptions {
	opts := BootstrapOptions{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--lang" && i+1 < len(args):
			opts.Lang = args[i+1]
			i++
		case strings.HasPrefix(arg, "--lang="):
			opts.Lang = strings.TrimPrefix(arg, "--lang=")
		case arg == "--config" && i+1 < len(args):
			opts.ConfigPath = args[i+1]
			i++
		case strings.HasPrefix(arg, "--config="):
			opts.ConfigPath = strings.TrimPrefix(arg, "--config=")
		}
	}
	return opts
}

func Load(opts BootstrapOptions) (*Config, Paths, error) {
	paths, err := resolvePaths(opts)
	if err != nil {
		return nil, Paths{}, err
	}

	if err := os.MkdirAll(paths.AppHome, 0o755); err != nil {
		return nil, Paths{}, core.NewActionError("mkdir_failed", "err.path.create_app_home", err, "err.path.check_permission", map[string]any{
			"path": paths.AppHome,
		}, nil)
	}
	if err := os.MkdirAll(paths.LogDir, 0o755); err != nil {
		return nil, Paths{}, core.NewActionError("mkdir_failed", "err.path.create_log_dir", err, "err.path.check_permission", map[string]any{
			"path": paths.LogDir,
		}, nil)
	}

	cfg := defaultConfig(paths)
	if _, err := os.Stat(paths.ConfigFile); os.IsNotExist(err) {
		if err := Save(paths.ConfigFile, cfg); err != nil {
			return nil, Paths{}, err
		}
	} else if err == nil {
		data, readErr := os.ReadFile(paths.ConfigFile)
		if readErr != nil {
			return nil, Paths{}, core.NewActionError("read_config_failed", "err.config.read", readErr, "err.config.check_path", map[string]any{
				"path": paths.ConfigFile,
			}, nil)
		}
		if len(data) > 0 {
			if unmarshalErr := json.Unmarshal(data, cfg); unmarshalErr != nil {
				return nil, Paths{}, core.NewActionError("parse_config_failed", "err.config.parse", unmarshalErr, "err.config.fix_format", map[string]any{
					"path": paths.ConfigFile,
				}, nil)
			}
		}
	} else {
		return nil, Paths{}, err
	}

	// 语言优先级遵循：命令参数 > 环境变量 > 配置文件 > 默认值。
	cfg.Language = ResolveLanguage(opts, cfg)
	return cfg, paths, nil
}

func Save(path string, cfg *Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return core.NewActionError("marshal_config_failed", "err.config.write", err, "", nil, nil)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return core.NewActionError("write_config_failed", "err.config.write", err, "err.path.check_permission", map[string]any{
			"path": path,
		}, nil)
	}
	return nil
}

func ResolveLanguage(opts BootstrapOptions, cfg *Config) string {
	if opts.Lang != "" {
		return opts.Lang
	}
	if env := os.Getenv("MIHOCTL_LANG"); env != "" {
		return env
	}
	if cfg != nil && cfg.Language != "" {
		return cfg.Language
	}
	return defaultLanguage
}

func resolvePaths(opts BootstrapOptions) (Paths, error) {
	execPath, err := os.Executable()
	if err != nil {
		execPath = ""
	}
	execDir := ""
	if execPath != "" {
		execDir = filepath.Dir(execPath)
	}
	cwd, err := os.Getwd()
	if err != nil {
		cwd = ""
	}

	configFile := opts.ConfigPath
	if configFile == "" {
		configFile = os.Getenv("MIHOCTL_CONFIG")
	}
	if configFile == "" {
		appHome, err := appHome()
		if err != nil {
			return Paths{}, err
		}
		configFile = filepath.Join(appHome, "config.json")
	}
	configFile = expandHome(configFile)
	appHome := filepath.Dir(configFile)
	return Paths{
		AppHome:    appHome,
		ConfigFile: configFile,
		StateFile:  filepath.Join(appHome, "state.json"),
		BinDir:     filepath.Join(appHome, "bin"),
		SubDir:     filepath.Join(appHome, "subscriptions"),
		LogDir:     filepath.Join(appHome, "logs"),
		LogFile:    filepath.Join(appHome, "logs", "mihomo.log"),
		ExecPath:   execPath,
		ExecDir:    execDir,
		CWD:        cwd,
	}, nil
}

func defaultConfig(paths Paths) *Config {
	mihomoDir := defaultMihomoDir()
	binaryPath := filepath.Join(paths.BinDir, "mihomo")
	return &Config{
		Language: defaultLanguage,
		Mode:     "env",
		Core: CoreConfig{
			Repo:              "MetaCubeX/mihomo",
			InstallDir:        paths.BinDir,
			DatabaseDir:       mihomoDir,
			AutoCheckUpdates:  true,
			CheckIntervalHour: 24,
		},
		Controller: Controller{
			Address: "http://127.0.0.1:9090",
		},
		Mihomo: Mihomo{
			BinaryPath: binaryPath,
			ConfigPath: filepath.Join(mihomoDir, "config.yaml"),
			WorkDir:    mihomoDir,
		},
		SystemProxy: SystemProxy{
			Host: "127.0.0.1",
			Port: 7890,
		},
		HealthCheck: HealthCheck{
			URL:       "https://www.gstatic.com/generate_204",
			TimeoutMS: 5000,
		},
		Subscriptions: []Subscription{},
	}
}

func appHome() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", core.NewActionError("user_config_dir_failed", "err.path.user_config_dir", err, "", nil, nil)
	}
	return filepath.Join(base, "mihoctl"), nil
}

func defaultMihomoDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "mihomo")
	default:
		return filepath.Join(home, ".config", "mihomo")
	}
}

func expandHome(path string) string {
	if path == "" || path[0] != '~' {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~/"))
}
