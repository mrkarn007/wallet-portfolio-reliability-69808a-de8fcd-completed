package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"keyrail.local/wallet-portfolio/internal/cache"
	"keyrail.local/wallet-portfolio/internal/models"
	"keyrail.local/wallet-portfolio/internal/provider"
)

var (
	ErrInvalidAddress      = errors.New("invalid wallet address")
	ErrProviderUnavailable = errors.New("provider unavailable")
)

type PortfolioService struct {
	provider        provider.MarketProvider
	cache           *cache.TTLCache
	priceTimeout    time.Duration
	maxPriceLookups int
}

func NewPortfolioService(provider provider.MarketProvider, cache *cache.TTLCache) *PortfolioService {
	return &PortfolioService{
		provider:        provider,
		cache:           cache,
		priceTimeout:    120 * time.Millisecond,
		maxPriceLookups: 5,
	}
}

func (s *PortfolioService) Summary(ctx context.Context, address string) (models.PortfolioResponse, error) {
	address = strings.ToLower(strings.TrimSpace(address))
	if !looksLikeAddress(address) {
		return models.PortfolioResponse{}, ErrInvalidAddress
	}

	balances, err := s.provider.WalletBalances(ctx, address)
	if err != nil {
		if errors.Is(err, provider.ErrWalletNotFound) {
			return models.PortfolioResponse{}, err
		}
		return models.PortfolioResponse{}, fmt.Errorf("load balances: %w", err)
	}

	response := models.PortfolioResponse{Address: address, Tokens: make([]models.TokenHolding, len(balances))}
	for i, balance := range balances {
		response.Tokens[i] = models.TokenHolding{
			Symbol: balance.Symbol,
			Name:   balance.Name,
			Chain:  balance.Chain,
			Amount: balance.Amount,
		}
	}

	type priceJob struct {
		index   int
		balance models.TokenBalance
	}

	jobs := make(chan priceJob)
	workerCount := min(s.maxPriceLookups, len(balances))
	var wg sync.WaitGroup
	wg.Add(workerCount)
	for range workerCount {
		go func() {
			defer wg.Done()
			for job := range jobs {
				balance := job.balance
				holding := &response.Tokens[job.index]

				cacheKey := strings.ToUpper(balance.Symbol)
				price, ok := s.cache.Get(cacheKey)
				if !ok {
					priceCtx, cancel := context.WithTimeout(ctx, s.priceTimeout)
					var priceErr error
					price, priceErr = s.provider.TokenPriceUSD(priceCtx, balance.Symbol)
					cancel()
					if priceErr != nil {
						continue
					}
					s.cache.Set(cacheKey, price)
				}

				value := balance.Amount * price
				holding.PriceUSD = &price
				holding.ValueUSD = &value
				holding.PriceAvailable = true
			}
		}()
	}

	for i, balance := range balances {
		jobs <- priceJob{index: i, balance: balance}
	}
	close(jobs)
	wg.Wait()

	for _, holding := range response.Tokens {
		if holding.ValueUSD != nil {
			response.TotalUSD += *holding.ValueUSD
		}
	}

	return response, nil
}

func looksLikeAddress(address string) bool {
	return strings.HasPrefix(address, "0x") && len(address) >= 6
}
