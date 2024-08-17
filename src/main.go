package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
	"unusual-api/src/config"
	"unusual-api/src/listener"
	"unusual-api/src/rpc"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"
)

var db *sql.DB
var poolData map[string]config.PoolInfo

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	// Initialize logger
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("Failed to load configuration", zap.Error(err))
	}

	// Load pool data
	poolData, err = loadPoolData()
	if err != nil {
		logger.Fatal("Failed to load pool data", zap.Error(err))
	}

	// Initialize Ethereum client
	client, err := rpc.NewClient(cfg.RPC_URL)
	if err != nil {
		logger.Fatal("Failed to connect to Ethereum client", zap.Error(err))
	}
	defer client.Close()

	// Initialize database
	db, err = initDB()
	if err != nil {
		logger.Fatal("Failed to initialize database", zap.Error(err))
	}
	defer db.Close()

	// Initialize event listener
	l := listener.New(client, cfg, logger, db, poolData)

	// Start listening for events
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	logger.Info("Starting listener...")
	go l.Start(ctx)

	// Start updating user balances daily
	go updateUserBalancesDaily(logger)

	// Initialize Gin router
	router := gin.Default()
	router.GET("/getUserPoint", getUserPointHandler)

	// Start the API server
	go func() {
		if err := router.Run(":8080"); err != nil {
			logger.Fatal("Failed to start API server", zap.Error(err))
		}
	}()

	// Wait for interrupt signal to gracefully shut down
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("Shutting down gracefully...")
}

func loadPoolData() (map[string]config.PoolInfo, error) {
	data, err := os.ReadFile("data/pool.json")
	if err != nil {
		return nil, err
	}

	var poolInfo struct {
		DataSources []config.PoolInfo `json:"dataSources"`
	}
	if err := json.Unmarshal(data, &poolInfo); err != nil {
		return nil, err
	}

	poolData := make(map[string]config.PoolInfo)
	for _, pool := range poolInfo.DataSources {
		poolData[pool.Address] = pool
	}

	return poolData, nil
}

func initDB() (*sql.DB, error) {
	db, err := sql.Open("sqlite3", "./user_balances.db")
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS user_balances (
		user_addr TEXT,
		contract_addr TEXT,
		balance TEXT,
		PRIMARY KEY (user_addr, contract_addr)
	)`)
	if err != nil {
		return nil, err
	}

	return db, nil
}

func updateUserBalancesDaily(logger *zap.Logger) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		<-ticker.C
		logger.Info("Updating user balances")
		// Implement the logic to update user balances here
	}
}

func getUserPointHandler(c *gin.Context) {
	userAddr := c.Query("userAddr")
	userMultiplier, err := strconv.ParseInt(c.Query("userMultiplier"), 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid userMultiplier"})
		return
	}

	points, err := calculateUserPoints(userAddr, userMultiplier)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to calculate user points"})
		return
	}

	c.JSON(200, gin.H{"userPoints": points.String()})
}

func calculateUserPoints(userAddr string, userMultiplier int64) (*big.Int, error) {
	totalPoints := big.NewInt(0)
	rows, err := db.Query("SELECT contract_addr, balance FROM user_balances WHERE user_addr = ?", userAddr)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var contractAddr, balanceStr string
		err := rows.Scan(&contractAddr, &balanceStr)
		if err != nil {
			return nil, err
		}

		balance, ok := new(big.Int).SetString(balanceStr, 10)
		if !ok {
			return nil, fmt.Errorf("invalid balance: %s", balanceStr)
		}

		poolInfo, ok := poolData[contractAddr]
		if !ok {
			continue
		}

		pillsMultiplier, _ := new(big.Int).SetString(poolInfo.PillsMultiplier, 10)
		points := new(big.Int).Mul(balance, pillsMultiplier)
		totalPoints.Add(totalPoints, points)
	}

	return new(big.Int).Mul(totalPoints, big.NewInt(userMultiplier)), nil
}
