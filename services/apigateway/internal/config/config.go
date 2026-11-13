package config

import (
	"fmt"

	"github.com/lisuhang11/SuIM/pkg/config"
	"github.com/lisuhang11/SuIM/pkg/register"
)

type VoicePreviewConfig struct {
	Python string `yaml:"python"`
	Script string `yaml:"script"`
}

type Config struct {
	ListenIP string `yaml:"listenIP"`
	Port     int    `yaml:"port"`
	// CompressionLevel matches OpenIM: -1 off, 0 default, 1 smallest, 2 fastest.
	CompressionLevel int                `yaml:"compressionLevel"`
	Registry         register.Config    `yaml:"registry"`
	Redis            config.Redis       `yaml:"redis"`
	VoicePreview     VoicePreviewConfig `yaml:"voicePreview"`
}

func (c *Config) Validate() error {
	if c.Port <= 0 {
		return fmt.Errorf("port is required")
	}
	switch c.CompressionLevel {
	case -1, 0, 1, 2:
	default:
		return fmt.Errorf("compressionLevel must be -1, 0, 1, or 2")
	}
	if c.ListenIP == "" {
		c.ListenIP = "0.0.0.0"
	}
	if c.Registry.Type == "" {
		return fmt.Errorf("registry type is required")
	}
	if c.VoicePreview.Python == "" {
		c.VoicePreview.Python = "/home/lisuhang/SuIM/agents/voice-assistant/.venv/bin/python"
	}
	if c.VoicePreview.Script == "" {
		c.VoicePreview.Script = "/home/lisuhang/SuIM/agents/voice-assistant/preview_tts.py"
	}
	return nil
}
