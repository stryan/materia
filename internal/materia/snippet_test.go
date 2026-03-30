package materia

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"primamateria.systems/materia/pkg/mocks"
)

func Test_secretEnv(t *testing.T) {
	hm := mocks.NewMockHostManager(t)
	hm.EXPECT().SecretName("mysecret").Return("materia-mysecret")
	hm.EXPECT().SecretName("db_password").Return("materia-db_password")

	macros := loadDefaultMacros(&MateriaConfig{}, hm, nil)(nil)

	tests := []struct {
		name string
		args []string
		want string
	}{
		{"no args", []string{}, ""},
		{"one arg", []string{"mysecret"}, "Secret=materia-mysecret,type=env,target=mysecret"},
		{"two args", []string{"db_password", "DB_PASS"}, "Secret=materia-db_password,type=env,target=DB_PASS"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := macros["secretEnv"].(func(...string) string)(tt.args...)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_secretMount(t *testing.T) {
	hm := mocks.NewMockHostManager(t)
	hm.EXPECT().SecretName("mysecret").Return("materia-mysecret")
	hm.EXPECT().SecretName("tls_cert").Return("materia-tls_cert")

	macros := loadDefaultMacros(&MateriaConfig{}, hm, nil)(nil)

	tests := []struct {
		name string
		args []string
		want string
	}{
		{"no args", []string{}, ""},
		{"one arg", []string{"mysecret"}, "Secret=materia-mysecret,type=mount,target=mysecret"},
		{"two args", []string{"tls_cert", "/etc/tls/cert.pem"}, "Secret=materia-tls_cert,type=mount,target=/etc/tls/cert.pem"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := macros["secretMount"].(func(...string) string)(tt.args...)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_m_default(t *testing.T) {
	hm := mocks.NewMockHostManager(t)
	macros := loadDefaultMacros(&MateriaConfig{}, hm, nil)(map[string]any{"existing": "value"})

	tests := []struct {
		name    string
		varName string
		defVal  string
		want    string
	}{
		{"existing var", "existing", "default", "value"},
		{"missing var", "missing", "default", "default"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := macros["m_default"].(func(string, string) string)(tt.varName, tt.defVal)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_exists(t *testing.T) {
	hm := mocks.NewMockHostManager(t)
	macros := loadDefaultMacros(&MateriaConfig{}, hm, nil)(map[string]any{"existing": "value"})

	tests := []struct {
		name    string
		varName string
		want    bool
	}{
		{"existing var", "existing", true},
		{"missing var", "missing", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := macros["exists"].(func(string) bool)(tt.varName)
			assert.Equal(t, tt.want, got)
		})
	}
}
