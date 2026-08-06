// update.go — provider-agnostic card database refresh orchestration.
package fetch

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/slh/riftport/internal/cards"
	"github.com/slh/riftport/internal/database"
)

// Provider returns a complete card snapshot from one remote source.
type Provider interface {
	Name() string
	FetchCards(context.Context) ([]cards.Card, error)
}

// ProviderFailure records a provider that failed before a fallback succeeded.
type ProviderFailure struct {
	Source string
	Err    error
}

// UpdateResult describes the source selected for a completed refresh.
type UpdateResult struct {
	Source   string
	Count    int
	Failures []ProviderFailure
}

// UpdateDB tries providers in order and atomically installs the first complete snapshot.
func UpdateDB(ctx context.Context, db *database.DB, providers ...Provider) (UpdateResult, error) {
	if len(providers) == 0 {
		return UpdateResult{}, fmt.Errorf("no card data providers configured")
	}

	result := UpdateResult{}
	var providerErrors []error
	for _, provider := range providers {
		snapshot, err := provider.FetchCards(ctx)
		if err != nil {
			result.Failures = append(result.Failures, ProviderFailure{Source: provider.Name(), Err: err})
			providerErrors = append(providerErrors, fmt.Errorf("%s: %w", provider.Name(), err))
			continue
		}
		if len(snapshot) == 0 {
			err := fmt.Errorf("empty card snapshot")
			result.Failures = append(result.Failures, ProviderFailure{Source: provider.Name(), Err: err})
			providerErrors = append(providerErrors, fmt.Errorf("%s: %w", provider.Name(), err))
			continue
		}

		now := time.Now().UTC()
		if err := db.ReplaceCards(ctx, snapshot, map[string]string{
			"last_updated": now.Format(time.RFC3339),
			"source":       provider.Name(),
		}); err != nil {
			return UpdateResult{}, fmt.Errorf("install %s card snapshot: %w", provider.Name(), err)
		}
		result.Source = provider.Name()
		result.Count = len(snapshot)
		return result, nil
	}
	return result, fmt.Errorf("all card data providers failed: %w", errors.Join(providerErrors...))
}
