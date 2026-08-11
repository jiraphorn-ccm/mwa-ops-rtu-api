package config

import (
	"fmt"
	"net"
	"net/url"
)

// DatabaseURL resolves the PostgreSQL connection string. When DATABASE_URL is
// set it wins; otherwise the URL is assembled from DB_HOST/DB_PORT/DB_USER/
// DB_PASSWORD/DB_NAME/DB_SSLMODE.
func (c *Config) resolveDatabaseURL() error {
	if c.DatabaseURL != "" {
		return nil
	}

	if c.DBHost == "" {
		return fmt.Errorf("DB_HOST is required when DATABASE_URL is not set")
	}
	if c.DBUser == "" {
		return fmt.Errorf("DB_USER is required when DATABASE_URL is not set")
	}
	if c.DBPassword == "" {
		return fmt.Errorf("DB_PASSWORD is required when DATABASE_URL is not set")
	}
	if c.DBName == "" {
		return fmt.Errorf("DB_NAME is required when DATABASE_URL is not set")
	}

	if c.DBPort == 0 {
		c.DBPort = 5432
	}
	if c.DBSSLMode == "" {
		if c.IsProduction() {
			c.DBSSLMode = "require"
		} else {
			c.DBSSLMode = "disable"
		}
	}

	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(c.DBUser, c.DBPassword),
		Host:   net.JoinHostPort(c.DBHost, fmt.Sprintf("%d", c.DBPort)),
		Path:   "/" + c.DBName,
	}
	q := u.Query()
	q.Set("sslmode", c.DBSSLMode)
	u.RawQuery = q.Encode()
	c.DatabaseURL = u.String()
	return nil
}
