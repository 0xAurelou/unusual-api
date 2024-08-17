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

type PoolInfo struct {
	Name            string  `json:"name"`
	Address         string  `json:"address"`
	StartBlock      uint64  `json:"startBlock"`
	PillsMultiplier string  `json:"pillsMultiplier"`
	EventHandlers   []Event `json:"eventHandlers"`
}

type Event struct {
	Event   string `json:"event"`
	Handler string `json:"handler"`
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
		Contracts:  loadContracts(),
	}, nil
}

func loadContracts() map[string]string {
	return map[string]string{
		"USD0++":                        "0x35D8949372D46B7a3D5A56006AE77B215fc69bC0",
		"USD0/USDC CurvePool":           "0x14100f81e33C33Ecc7CDac70181Fb45B6E78569F",
		"USD0/USDC CurveGauge":          "0xC2a56E8888786A30A5b56Cbe4450A81DDF26aC0C",
		"USD0/USD0++ CurvePool":         "0x1d08E7adC263CfC70b1BaBe6dC5Bb339c16Eec52",
		"USD0/USD0++ CurveGauge":        "0x5C00817B67b40f3b347bD4275B4BBA4840c8127a",
		"USD0++/USDC Morpho Market":     "0xBBBBBbbBBb9cC5e90e3b3Af64bdAF62C37EEFFCb",
		"USD0USD0++/USDC Morpho Market": "0xBBBBBbbBBb9cC5e90e3b3Af64bdAF62C37EEFFCb",
		"USDC Morpho Vault":             "0xd63070114470f685b75B74D60EEc7c1113d33a3D",
	}
}
