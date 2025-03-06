package git

import "github.com/knadh/koanf/v2"

type Config struct {
	PrivateKey string `koanf:"privatekey"`
	Username   string
	Password   string
	Insecure   bool `koanf:"insecure"`
}

func NewConfig(k *koanf.Koanf) (*Config, error) {
	var c Config
	c.PrivateKey = k.String("privatekey")
	c.Insecure = k.Bool("insecure")
	c.Username = k.String("username")
	c.Password = k.String("password")
	return &c, nil
}
