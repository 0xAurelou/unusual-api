package listener

import (
	"context"
	"database/sql"
	"fmt"
	"math/big"
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
	client   *rpc.Client
	config   *config.Config
	logger   *zap.Logger
	db       *sql.DB
	poolData map[string]config.PoolInfo
}

const transferEventABI = `[{"anonymous":false,"inputs":[{"indexed":true,"name":"from","type":"address"},{"indexed":true,"name":"to","type":"address"},{"indexed":false,"name":"value","type":"uint256"}],"name":"Transfer","type":"event"}]`

func New(client *rpc.Client, config *config.Config, logger *zap.Logger, db *sql.DB, poolData map[string]config.PoolInfo) *Listener {
	return &Listener{
		client:   client,
		config:   config,
		logger:   logger,
		db:       db,
		poolData: poolData,
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
				if err := l.handleLog(name, address, vLog); err != nil {
					l.logger.Error("Error handling log", zap.Error(err))
				}
			}

			currentBlock = endBlock + 1
		}
	}
}

func (l *Listener) handleLog(name, address string, vLog types.Log) error {
	if len(vLog.Topics) != 3 || vLog.Topics[0] != common.HexToHash("0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef") {
		return nil // Not a Transfer event, skip
	}

	contractAbi, err := abi.JSON(strings.NewReader(transferEventABI))
	if err != nil {
		return fmt.Errorf("error parsing ABI: %v", err)
	}

	var event struct {
		From  common.Address
		To    common.Address
		Value *big.Int
	}

	err = contractAbi.UnpackIntoInterface(&event, "Transfer", vLog.Data)
	if err != nil {
		return fmt.Errorf("error unpacking event data: %v", err)
	}

	event.From = common.BytesToAddress(vLog.Topics[1].Bytes())
	event.To = common.BytesToAddress(vLog.Topics[2].Bytes())

	l.logger.Info("Transfer event detected",
		zap.String("contract", name),
		zap.String("from", event.From.Hex()),
		zap.String("to", event.To.Hex()),
		zap.String("value", event.Value.String()),
	)

	err = l.updateUserBalance(event.From.Hex(), address, event.Value, false)
	if err != nil {
		l.logger.Error("Failed to update sender balance", zap.Error(err))
	}

	err = l.updateUserBalance(event.To.Hex(), address, event.Value, true)
	if err != nil {
		l.logger.Error("Failed to update receiver balance", zap.Error(err))
	}

	return nil
}

func (l *Listener) updateUserBalance(userAddr, contractAddr string, value *big.Int, isReceiver bool) error {
	tx, err := l.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var currentBalanceStr string
	err = tx.QueryRow("SELECT balance FROM user_balances WHERE user_addr = ? AND contract_addr = ?", userAddr, contractAddr).Scan(&currentBalanceStr)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	// Initialize currentBalance
	currentBalance := big.NewInt(0)
	if err == nil && currentBalanceStr != "" {
		if _, ok := currentBalance.SetString(currentBalanceStr, 10); !ok {
			return fmt.Errorf("invalid balance string: %s", currentBalanceStr)
		}
	}

	// Calculate the new balance
	newBalance := new(big.Int)
	if isReceiver {
		newBalance.Add(currentBalance, value)
	} else {
		if currentBalance.Cmp(value) < 0 {
			// If the current balance is less than the value to subtract, set to 0
			newBalance.SetInt64(0)
		} else {
			newBalance.Sub(currentBalance, value)
		}
	}

	_, err = tx.Exec(`
        INSERT INTO user_balances (user_addr, contract_addr, balance)
        VALUES (?, ?, ?)
        ON CONFLICT(user_addr, contract_addr) DO UPDATE SET
        balance = ?
    `, userAddr, contractAddr, newBalance.String(), newBalance.String())
	if err != nil {
		return err
	}

	return tx.Commit()
}
