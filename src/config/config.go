package config

import (
	"errors"
	"os"
	"strconv"
)

type Config struct {
	RPC_URL    string
	StartBlock uint64
	Contracts  map[string]string
}

func Load() (*Config, error) {
	rpcURL := os.Getenv("RPC_URL")
	if rpcURL == "" {
		return nil, errors.New("RPC_URL not set")
	}

	startBlockStr := os.Getenv("START_BLOCK")
	startBlock, err := strconv.ParseUint(startBlockStr, 10, 64)
	if err != nil {
		return nil, errors.New("invalid START_BLOCK")
	}

	return &Config{
		RPC_URL:    rpcURL,
		StartBlock: startBlock,
		Contracts: map[string]string{
			"USD0++": "0x35D8949372D46B7a3D5A56006AE77B215fc69bC0",
			// Add more contracts here
		},
	}, nil
}
