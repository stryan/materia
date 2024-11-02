package materia

import "errors"

type Config struct {
	GitRepo          string `envconfig:"GIT_REPO"`
	Debug            bool   `envconfig:"DEBUG"`
	Timeout          int
	Secrets          string
	SecretsAgeIdents string `envconfig:"SECRETS_AGE_IDENTS"`
}

func (c *Config) Validate() error {
	if c.GitRepo == "" {
		return errors.New("need git repo location")
	}
	return nil
}
