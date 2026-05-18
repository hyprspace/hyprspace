package dns

import (
	"testing"

	"github.com/hyprspace/hyprspace/config"
	"github.com/stretchr/testify/assert"
)

func TestT22_domainSuffix_hyprspace(t *testing.T) {
	cfg := config.Config{Interface: "hyprspace"}
	assert.Equal(t, "hyprspace.", domainSuffix(cfg))
}

func TestT22_domainSuffix_custom(t *testing.T) {
	cfg := config.Config{Interface: "hs0"}
	assert.Equal(t, "hs0.hyprspace.", domainSuffix(cfg))
}

func TestT22_domainSuffix_other(t *testing.T) {
	cfg := config.Config{Interface: "myvpn"}
	assert.Equal(t, "myvpn.hyprspace.", domainSuffix(cfg))
}

func TestT22_domainSuffix_empty(t *testing.T) {
	cfg := config.Config{Interface: ""}
	assert.Equal(t, ".hyprspace.", domainSuffix(cfg))
}

func TestT23_withDomainSuffix(t *testing.T) {
	cfg := config.Config{Interface: "hyprspace"}
	assert.Equal(t, "alice.hyprspace.", withDomainSuffix(cfg, "alice"))
}

func TestT23_withDomainSuffix_customInterface(t *testing.T) {
	cfg := config.Config{Interface: "hs0"}
	assert.Equal(t, "bob.hs0.hyprspace.", withDomainSuffix(cfg, "bob"))
}

func TestT23_withDomainSuffix_servicePrefix(t *testing.T) {
	cfg := config.Config{Interface: "hs0"}
	assert.Equal(t, "http.alice.hs0.hyprspace.", withDomainSuffix(cfg, "http.alice"))
}
