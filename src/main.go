package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/sha3"
)

const (
	userAddress = "0x654679a6587cd6821Ba6dcf44B2647D4ae815847"
	startBlock  = 20276442
	// Example start block
)

var (
	client *ethclient.Client
)

type TokenJsonDataStruct struct {
	DataSources []struct {
		Name          string `json:"name"`
		Address       string `json:"address"`
		StartBlock    int    `json:"startBlock"`
		EventHandlers []struct {
			Event   string `json:"event"`
			Handler string `json:"handler"`
		} `json:"eventHandlers"`
	} `json:"dataSources"`
}

func main() {
	// Load environment variables from .env file
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file", err)
	}

	rpcURL := os.Getenv("RPC_URL")

	var rpcError error

	client, rpcError = ethclient.Dial(rpcURL)
	if rpcError != nil {
		log.Fatalf("Failed to connect to Ethereum client: %v", err)
	}

	fmt.Println("Connected to Ethereum client")

	// Event listeners
	listenToEvents()

	defer client.Close()

}

func listenToEvents() {
	// Subscribe to events for each data source
	subscribeToEvents("USD0++", "0x35D8949372D46B7a3D5A56006AE77B215fc69bC0", startBlock)
}

func subscribeToEvents(name string, contractAddress string, startBlock uint64) {
	fmt.Printf("Polling for events for %s at %s\n", name, contractAddress)

	currentBlock := startBlock
	for {
		latestBlock, err := client.BlockNumber(context.Background())
		if err != nil {
			log.Printf("Error getting latest block number: %v", err)
			time.Sleep(10 * time.Second)
			continue
		}

		if latestBlock == currentBlock {
			time.Sleep(13 * time.Second)
			continue
		}

		// Query in smaller chunks (e.g., 5000 blocks at a time)
		for currentBlock <= latestBlock {
			endBlock := currentBlock + 5000
			if endBlock > latestBlock {
				endBlock = latestBlock
			}

			query := ethereum.FilterQuery{
				Addresses: []common.Address{common.HexToAddress(contractAddress)},
				FromBlock: big.NewInt(int64(currentBlock)),
				ToBlock:   big.NewInt(int64(endBlock)),
			}

			logs, err := client.FilterLogs(context.Background(), query)
			if err != nil {
				log.Printf("Error filtering logs: %v", err)
				time.Sleep(10 * time.Second)
				break
			}

			for _, vLog := range logs {
				// Open our jsonFile
				jsonData, err := os.ReadFile("data/usd0pp.json")
				if err != nil {
					fmt.Println(err)
					continue
				}
				handleLog(vLog, jsonData)
			}

			currentBlock = endBlock + 1
		}

	}
}

func handleLog(vLog types.Log, jsonData []byte) {
	var deserializedData TokenJsonDataStruct
	json.Unmarshal(jsonData, &deserializedData)

	for _, dataSource := range deserializedData.DataSources {
		for _, eventHandler := range dataSource.EventHandlers {
			fmt.Println("vLog topic:", vLog.Topics[0].Hex())

			// Hash the event signature
			hash := sha3.NewLegacyKeccak256()
			hash.Write([]byte(eventHandler.Event))
			eventSignature := hash.Sum(nil)

			// Convert to hexadecimal string with "0x" prefix
			hexSignature := "0x" + hex.EncodeToString(eventSignature)
			fmt.Println("Computed signature:", hexSignature)

			if strings.EqualFold(vLog.Topics[0].Hex(), hexSignature) {
				fmt.Println("Found event:", eventHandler.Event)

				// Define the ABI for the Transfer event
				transferEventABI := `[{"type":"event","name":"Transfer","inputs":[{"type":"address","name":"from","indexed":true},{"type":"address","name":"to","indexed":true},{"type":"uint256","name":"value","indexed":false}],"anonymous":false}]`

				parsedABI, err := abi.JSON(strings.NewReader(transferEventABI))
				if err != nil {
					fmt.Println("Error parsing ABI:", err)
					return
				}

				// Decode the event data
				event := struct {
					From  common.Address
					To    common.Address
					Value *big.Int
				}{}

				err = parsedABI.UnpackIntoInterface(&event, "Transfer", vLog.Data)
				if err != nil {
					fmt.Println("Error unpacking event data:", err)
					return
				}

				// Convert the value to a float64 with 18 decimal places
				fValue := new(big.Float).SetInt(event.Value)
				divisor := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
				result, _ := new(big.Float).Quo(fValue, divisor).Float64()

				fmt.Printf("Transfer value: %.2f\n", result)
			}

		}
	}
}
