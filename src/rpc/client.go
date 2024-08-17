package rpc

import (
	"context"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

type Client struct {
	*ethclient.Client
	rpcClient *rpc.Client
	url       string
}

const (
	maxRetries = 3
	retryDelay = 2 * time.Second
)

func NewClient(rpcURL string) (*Client, error) {
	var client *Client
	var err error

	for i := 0; i < maxRetries; i++ {
		client, err = attemptConnection(rpcURL)
		if err == nil {
			return client, nil
		}
		time.Sleep(retryDelay)
	}

	return nil, fmt.Errorf("failed to connect after %d attempts: %v", maxRetries, err)
}

func attemptConnection(rpcURL string) (*Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rpcClient, err := rpc.DialContext(ctx, rpcURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RPC: %v", err)
	}

	ethClient := ethclient.NewClient(rpcClient)

	// Perform a test call to ensure the connection is working
	_, err = ethClient.BlockNumber(ctx)
	if err != nil {
		rpcClient.Close()
		return nil, fmt.Errorf("failed to retrieve block number: %v", err)
	}

	return &Client{
		Client:    ethClient,
		rpcClient: rpcClient,
		url:       rpcURL,
	}, nil
}

func (c *Client) Close() {
	if c.rpcClient != nil {
		c.rpcClient.Close()
	}
}

func (c *Client) Reconnect() error {
	c.Close()

	newClient, err := NewClient(c.url)
	if err != nil {
		return fmt.Errorf("failed to reconnect: %v", err)
	}

	*c = *newClient
	return nil
}
