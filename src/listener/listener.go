package listener

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"go.uber.org/zap"

	"unusual-api/src/config"
	"unusual-api/src/rpc"
)

type Listener struct {
	client *rpc.Client
	config *config.Config
	logger *zap.Logger
}

func New(client *rpc.Client, config *config.Config, logger *zap.Logger) *Listener {
	return &Listener{
		client: client,
		config: config,
		logger: logger,
	}
}

func (l *Listener) Start(ctx context.Context) {
	for name, address := range l.config.Contracts {
		go l.listenToEvents(ctx, name, address)
	}
}

func (l *Listener) listenToEvents(ctx context.Context, name, address string) {
	currentBlock := l.config.StartBlock
	contractAddress := common.HexToAddress(address)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			latestBlock, err := l.client.BlockNumber(ctx)
			if err != nil {
				l.logger.Error("Error getting latest block number", zap.Error(err))
				time.Sleep(10 * time.Second)
				continue
			}

			if latestBlock <= currentBlock {
				time.Sleep(13 * time.Second)
				continue
			}

			endBlock := currentBlock + 5000
			if endBlock > latestBlock {
				endBlock = latestBlock
			}

			query := ethereum.FilterQuery{
				Addresses: []common.Address{contractAddress},
				FromBlock: big.NewInt(int64(currentBlock)),
				ToBlock:   big.NewInt(int64(endBlock)),
			}

			logs, err := l.client.FilterLogs(ctx, query)
			if err != nil {
				l.logger.Error("Error filtering logs", zap.Error(err))
				time.Sleep(10 * time.Second)
				continue
			}

			for _, vLog := range logs {
				if err := l.handleLog(name, vLog); err != nil {
					l.logger.Error("Error handling log", zap.Error(err))
				}
			}

			currentBlock = endBlock + 1
		}
	}
}

func (l *Listener) handleLog(name string, vLog types.Log) error {
	// Implement log handling logic here
	// This could involve decoding the log data and calling appropriate handler functions
	fmt.Println(name, vLog)

	return nil
}
