package app

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type HostConfig struct {
	Name     string `yaml:"name"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Auth     string `yaml:"auth"`    // "private_key" | "password"
	KeyPath  string `yaml:"keyPath"` // 本机上的私钥路径
	Password string `yaml:"password"`
	Remark   string `yaml:"remark"`

	LanHost string `yaml:"lanHost"`
	LanPort int    `yaml:"lanPort"`
}

// LoadHosts 读取 YAML 主机配置
func LoadHosts(path string) ([]HostConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read hosts yaml: %w", err)
	}
	var hosts []HostConfig
	if err := yaml.Unmarshal(data, &hosts); err != nil {
		return nil, fmt.Errorf("unmarshal hosts yaml: %w", err)
	}
	return hosts, nil
}
