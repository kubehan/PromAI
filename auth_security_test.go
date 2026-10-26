package main

import (
	"testing"

	"PromAI/pkg/config"
)

func TestResolveJWTSecretRejectsInsecureValues(t *testing.T) {
	cases := []struct {
		name   string
		env    string
		config string
	}{
		{name: "missing", env: "", config: ""},
		{name: "published config default", env: "", config: "promai-jwt-secret-change-in-production"},
		{name: "published env default", env: "promai-default-secret", config: ""},
		{name: "k8s example default", env: "", config: "change-me-in-production"},
		{name: "too short", env: "", config: "short-secret"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("PROMAI_JWT_SECRET", tc.env)
			cfg := &config.Config{}
			cfg.Auth.JWTSecret = tc.config
			if err := resolveJWTSecret(cfg); err == nil {
				t.Fatal("expected insecure JWT secret to be rejected")
			}
		})
	}
}

func TestResolveJWTSecretPrefersEnv(t *testing.T) {
	envSecret := "0123456789abcdef0123456789abcdef"
	t.Setenv("PROMAI_JWT_SECRET", envSecret)
	cfg := &config.Config{}
	cfg.Auth.JWTSecret = "another-secret-that-is-long-enough-123"

	if err := resolveJWTSecret(cfg); err != nil {
		t.Fatalf("expected env secret to be accepted: %v", err)
	}
	if cfg.Auth.JWTSecret != envSecret {
		t.Fatalf("expected env secret to override config, got %q", cfg.Auth.JWTSecret)
	}
}

func TestNewAdminAPIRejectsInsecureConfiguredSecret(t *testing.T) {
	cfg := &config.Config{}
	cfg.Auth.JWTSecret = "promai-jwt-secret-change-in-production"

	api := NewAdminAPI(nil, cfg, nil)
	if api.jwtSecret != "" {
		t.Fatalf("insecure secret must not be used by AdminAPI, got %q", api.jwtSecret)
	}
}

func TestNewAdminAPIAcceptsStrongSecret(t *testing.T) {
	const secret = "0123456789abcdef0123456789abcdef"
	cfg := &config.Config{}
	cfg.Auth.JWTSecret = secret

	api := NewAdminAPI(nil, cfg, nil)
	if api.jwtSecret != secret {
		t.Fatalf("expected strong secret to be used, got %q", api.jwtSecret)
	}
}

func TestForgedDefaultTokenIsRejected(t *testing.T) {
	const realSecret = "0123456789abcdef0123456789abcdef"

	forged, err := generateToken("admin", "promai-jwt-secret-change-in-production")
	if err != nil {
		t.Fatalf("generate forged token: %v", err)
	}
	if _, err := validateToken(forged, realSecret); err == nil {
		t.Fatal("token signed with the published default secret must not validate")
	}

	valid, err := generateToken("admin", realSecret)
	if err != nil {
		t.Fatalf("generate valid token: %v", err)
	}
	claims, err := validateToken(valid, realSecret)
	if err != nil {
		t.Fatalf("valid token was rejected: %v", err)
	}
	if claims.Username != "admin" {
		t.Fatalf("unexpected username: %q", claims.Username)
	}
}
