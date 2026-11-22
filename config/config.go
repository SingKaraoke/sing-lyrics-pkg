// Package config loads process configuration from environment variables,
// shared by every service so a setting only has one name across the codebase.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// DefaultR2Account is the name of the account configured via the unsuffixed
// R2_ACCOUNT_ID / R2_ACCESS_KEY_ID / R2_SECRET_ACCESS_KEY / R2_BUCKET_NAME /
// R2_JURISDICTION variables - every deployment has at least this one.
const DefaultR2Account = "default"

// R2Account is one set of R2 credentials + bucket. Lyrics files are spread
// across however many of these are configured (see docs/ARCHITECTURE.md) to
// balance load and stay under each Cloudflare account's free-tier Class A
// operations quota; which account a given file lives in is recorded
// per-row in lyrics.storage_account.
type R2Account struct {
	Name            string
	AccountID       string
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
	Jurisdiction    string // optional, e.g. "eu"
	MaxUploads      int64  // optional; informational cap enforced by migration/upload_to_r2.py, unused by the Go services
}

// Endpoint builds the S3-compatible endpoint host for this account, honoring
// a data-residency jurisdiction if one is set (see docs/ARCHITECTURE.md §10).
func (a R2Account) Endpoint() string {
	// Self-hosting: any S3-compatible store (MinIO, Garage) instead of Cloudflare R2.
	if e := os.Getenv("S3_ENDPOINT"); e != "" {
		return e
	}
	if a.Jurisdiction != "" {
		return fmt.Sprintf("%s.%s.r2.cloudflarestorage.com", a.AccountID, a.Jurisdiction)
	}
	return fmt.Sprintf("%s.r2.cloudflarestorage.com", a.AccountID)
}

type Config struct {
	DatabaseURL string

	// R2Accounts is keyed by account name (DefaultR2Account plus whatever's
	// listed in R2_ACCOUNTS). lyrics-api needs every account's credentials
	// to serve downloads for files stored anywhere; upload-api only ever
	// writes new lyrics to R2WriteAccount.
	R2Accounts     map[string]R2Account
	R2WriteAccount string

	MeiliURL string
	MeiliKey string

	HTTPAddr string

	// Upload limits & security
	MaxUploadBytes   int64
	SignatureMaxSkew time.Duration
}

func Load() (*Config, error) {
	cfg := &Config{
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		R2Accounts:     loadR2Accounts(),
		R2WriteAccount: getEnvDefault("R2_WRITE_ACCOUNT", DefaultR2Account),
		MeiliURL:       getEnvDefault("MEILI_URL", "http://localhost:7700"),
		MeiliKey:       os.Getenv("MEILI_MASTER_KEY"),
		HTTPAddr:       getEnvDefault("HTTP_ADDR", ":8080"),
		MaxUploadBytes: getEnvInt64Default("MAX_UPLOAD_BYTES", 2<<20), // 2MB
	}

	skewSeconds := getEnvInt64Default("SIGNATURE_MAX_SKEW_SECONDS", 300)
	cfg.SignatureMaxSkew = time.Duration(skewSeconds) * time.Second

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if _, ok := cfg.R2Accounts[cfg.R2WriteAccount]; !ok {
		return nil, fmt.Errorf("R2_WRITE_ACCOUNT %q has no matching R2 credentials configured (checked R2_ACCOUNTS)", cfg.R2WriteAccount)
	}
	return cfg, nil
}

// loadR2Accounts builds DefaultR2Account from the unsuffixed R2_* vars
// (unconditionally, for backward compatibility), plus one extra account per
// name listed in R2_ACCOUNTS (comma-separated), each read from
// R2_ACCOUNT_<NAME>_* vars, e.g. R2_ACCOUNTS=secondary reads
// R2_ACCOUNT_SECONDARY_ACCOUNT_ID, R2_ACCOUNT_SECONDARY_ACCESS_KEY_ID, etc.
func loadR2Accounts() map[string]R2Account {
	accounts := map[string]R2Account{
		DefaultR2Account: {
			Name:            DefaultR2Account,
			AccountID:       os.Getenv("R2_ACCOUNT_ID"),
			AccessKeyID:     os.Getenv("R2_ACCESS_KEY_ID"),
			SecretAccessKey: os.Getenv("R2_SECRET_ACCESS_KEY"),
			Bucket:          os.Getenv("R2_BUCKET_NAME"),
			Jurisdiction:    os.Getenv("R2_JURISDICTION"),
			MaxUploads:      getEnvInt64Default("R2_MAX_UPLOADS", 0),
		},
	}

	for _, name := range splitCSV(os.Getenv("R2_ACCOUNTS")) {
		prefix := "R2_ACCOUNT_" + strings.ToUpper(name) + "_"
		accounts[name] = R2Account{
			Name:            name,
			AccountID:       os.Getenv(prefix + "ACCOUNT_ID"),
			AccessKeyID:     os.Getenv(prefix + "ACCESS_KEY_ID"),
			SecretAccessKey: os.Getenv(prefix + "SECRET_ACCESS_KEY"),
			Bucket:          os.Getenv(prefix + "BUCKET_NAME"),
			Jurisdiction:    os.Getenv(prefix + "JURISDICTION"),
			MaxUploads:      getEnvInt64Default(prefix+"MAX_UPLOADS", 0),
		}
	}
	return accounts
}

func splitCSV(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func getEnvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt64Default(key string, def int64) int64 {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return def
	}
	return n
}
