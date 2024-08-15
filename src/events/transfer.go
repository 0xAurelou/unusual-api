package events

import (
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

var TransferEventABI = `[{"type":"event","name":"Transfer","inputs":[{"type":"address","name":"from","indexed":true},{"type":"address","name":"to","indexed":true},{"type":"uint256","name":"value","indexed":false}],"anonymous":false}]`

type TransferEvent struct {
	From  common.Address
	To    common.Address
	Value *big.Int
}

func DecodeTransferEvent(log types.Log) (*TransferEvent, error) {
	event := new(TransferEvent)
	parsedABI, err := abi.JSON(strings.NewReader(TransferEventABI))
	if err != nil {
		return nil, err
	}

	err = parsedABI.UnpackIntoInterface(event, "Transfer", log.Data)
	if err != nil {
		return nil, err
	}

	event.From = common.BytesToAddress(log.Topics[1].Bytes())
	event.To = common.BytesToAddress(log.Topics[2].Bytes())

	return event, nil
}
