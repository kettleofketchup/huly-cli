package cmd

import (
	"errors"
	"fmt"
	"time"

	"github.com/kettleofketchup/huly-cli/src/huly/internal/creds"
	"github.com/kettleofketchup/huly-cli/src/huly/internal/huly"
)

// newClient loads credentials and constructs a REST client.
func newClient() (*huly.RestClient, creds.Credentials, error) {
	c, err := creds.Load()
	if err != nil {
		return nil, creds.Credentials{}, err
	}
	return huly.NewRestClient(c.Endpoint, c.Workspace, c.Token), c, nil
}

// mapAuthErr turns a 401 into actionable guidance (kept wrapped so callers can
// still errors.Is it back to huly.ErrUnauthorized).
func mapAuthErr(err error) error {
	if errors.Is(err, huly.ErrUnauthorized) {
		return fmt.Errorf("session expired or token invalid — run `huly login`: %w", err)
	}
	return err
}

// nowMillis is the current Unix time in milliseconds (tx timestamps).
func nowMillis() int64 { return time.Now().UnixMilli() }
