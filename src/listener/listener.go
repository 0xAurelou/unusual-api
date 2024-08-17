package listener

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"strings"
	"time"

	"unusual-api/src/config"
	"unusual-api/src/rpc"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"go.uber.org/zap"
)

type Listener struct {
	client *rpc.Client
	config *config.Config
	logger *zap.Logger
}

// Define the Transfer event ABI
const transferEventABI = `[{"anonymous":false,"inputs":[{"indexed":true,"name":"from","type":"address"},{"indexed":true,"name":"to","type":"address"},{"indexed":false,"name":"value","type":"uint256"}],"name":"Transfer","type":"event"}]`

func New(client *rpc.Client, config *config.Config, logger *zap.Logger) *Listener {
	return &Listener{
		client: client,
		config: config,
		logger: logger,
	}
}

func (l *Listener) Start(ctx context.Context) <-chan int {
	resultChan := make(chan int)
	for name, address := range l.config.Contracts {
		go l.listenToEvents(ctx, name, address, resultChan)
	}
	return resultChan
}

func (l *Listener) listenToEvents(ctx context.Context, name, address string, resultChan chan<- int) {
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
			endBlock := currentBlock + 10000
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
	// Check if the log is a Transfer event
	if len(vLog.Topics) != 3 || vLog.Topics[0] != common.HexToHash("0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef") {
		return nil // Not a Transfer event, skip
	}

	// Parse the ABI
	contractAbi, err := abi.JSON(strings.NewReader(transferEventABI))
	if err != nil {
		return fmt.Errorf("error parsing ABI: %v", err)
	}

	// Unpack the event data
	var event struct {
		From  common.Address
		To    common.Address
		Value *big.Int
	}
	err = contractAbi.UnpackIntoInterface(&event, "Transfer", vLog.Data)
	if err != nil {
		return fmt.Errorf("error unpacking event data: %v", err)
	}

	// Get the 'from' and 'to' addresses from the indexed topics
	event.From = common.BytesToAddress(vLog.Topics[1].Bytes())
	event.To = common.BytesToAddress(vLog.Topics[2].Bytes())

	if event.From == common.HexToAddress(os.Getenv("USER_ADDR")) || event.To == common.HexToAddress(os.Getenv("USER_ADDR")) {
		// Log the transfer details
		l.logger.Info("Transfer event detected",
			zap.String("contract", name),
			zap.String("from", event.From.Hex()),
			zap.String("to", event.To.Hex()),
			zap.String("value", event.Value.String()),
		)

		return nil
	}

	return nil
}
