package internal

// UnversionedTeleport is used to read all versions of teleport.yaml, including
// versions that may now be unsupported.
type UnversionedTeleport struct {
	Teleport UnversionedConfig `yaml:"teleport"`
}

// UnversionedConfig is used to read unversioned configuration from teleport and tbot.
type UnversionedConfig struct {
	AuthServers []string `yaml:"auth_servers"`
	AuthServer  string   `yaml:"auth_server"`
	ProxyServer string   `yaml:"proxy_server"`
	DataDir     string   `yaml:"data_dir"`
}
