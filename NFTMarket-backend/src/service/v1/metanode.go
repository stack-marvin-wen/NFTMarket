package service

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/ProjectsTask/EasySwapBackend/src/service/svc"
	typesv1 "github.com/ProjectsTask/EasySwapBackend/src/types/v1"
	"github.com/ProjectsTask/EasySwapBase/logger/xzap"
	"github.com/ProjectsTask/EasySwapBase/stores/gdb/orderbookmodel/multi"
)

// MetaNodeNFT合约完整ABI（基于提供的Solidity代码生成）
const MetaNodeNFTABI = `[
	{
		"inputs": [
			{"internalType": "address", "name": "initialOwner", "type": "address"}
		],
		"stateMutability": "nonpayable",
		"type": "constructor"
	},
	{
		"inputs": [
			{"internalType": "address", "name": "sender", "type": "address"},
			{"internalType": "uint256", "name": "tokenId", "type": "uint256"},
			{"internalType": "address", "name": "owner", "type": "address"}
		],
		"name": "ERC721IncorrectOwner",
		"type": "error"
	},
	{
		"inputs": [
			{"internalType": "address", "name": "operator", "type": "address"},
			{"internalType": "uint256", "name": "tokenId", "type": "uint256"}
		],
		"name": "ERC721InsufficientApproval",
		"type": "error"
	},
	{
		"inputs": [
			{"internalType": "address", "name": "approver", "type": "address"}
		],
		"name": "ERC721InvalidApprover",
		"type": "error"
	},
	{
		"inputs": [
			{"internalType": "address", "name": "operator", "type": "address"}
		],
		"name": "ERC721InvalidOperator",
		"type": "error"
	},
	{
		"inputs": [
			{"internalType": "address", "name": "owner", "type": "address"}
		],
		"name": "ERC721InvalidOwner",
		"type": "error"
	},
	{
		"inputs": [
			{"internalType": "address", "name": "receiver", "type": "address"}
		],
		"name": "ERC721InvalidReceiver",
		"type": "error"
	},
	{
		"inputs": [
			{"internalType": "address", "name": "sender", "type": "address"}
		],
		"name": "ERC721InvalidSender",
		"type": "error"
	},
	{
		"inputs": [
			{"internalType": "uint256", "name": "tokenId", "type": "uint256"}
		],
		"name": "ERC721NonexistentToken",
		"type": "error"
	},
	{
		"inputs": [
			{"internalType": "address", "name": "owner", "type": "address"}
		],
		"name": "OwnableInvalidOwner",
		"type": "error"
	},
	{
		"inputs": [
			{"internalType": "address", "name": "account", "type": "address"}
		],
		"name": "OwnableUnauthorizedAccount",
		"type": "error"
	},
	{
		"anonymous": false,
		"inputs": [
			{"indexed": true, "internalType": "address", "name": "owner", "type": "address"},
			{"indexed": true, "internalType": "address", "name": "approved", "type": "address"},
			{"indexed": true, "internalType": "uint256", "name": "tokenId", "type": "uint256"}
		],
		"name": "Approval",
		"type": "event"
	},
	{
		"anonymous": false,
		"inputs": [
			{"indexed": true, "internalType": "address", "name": "owner", "type": "address"},
			{"indexed": true, "internalType": "address", "name": "operator", "type": "address"},
			{"indexed": false, "internalType": "bool", "name": "approved", "type": "bool"}
		],
		"name": "ApprovalForAll",
		"type": "event"
	},
	{
		"anonymous": false,
		"inputs": [
			{"indexed": true, "internalType": "address", "name": "previousOwner", "type": "address"},
			{"indexed": true, "internalType": "address", "name": "newOwner", "type": "address"}
		],
		"name": "OwnershipTransferred",
		"type": "event"
	},
	{
		"anonymous": false,
		"inputs": [
			{"indexed": true, "internalType": "address", "name": "from", "type": "address"},
			{"indexed": true, "internalType": "address", "name": "to", "type": "address"},
			{"indexed": true, "internalType": "uint256", "name": "tokenId", "type": "uint256"}
		],
		"name": "Transfer",
		"type": "event"
	},
	{
		"inputs": [
			{"internalType": "address", "name": "to", "type": "address"},
			{"internalType": "uint256", "name": "tokenId", "type": "uint256"}
		],
		"name": "approve",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [
			{"internalType": "address", "name": "owner", "type": "address"}
		],
		"name": "balanceOf",
		"outputs": [
			{"internalType": "uint256", "name": "", "type": "uint256"}
		],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [
			{"internalType": "uint256", "name": "tokenId", "type": "uint256"}
		],
		"name": "getApproved",
		"outputs": [
			{"internalType": "address", "name": "", "type": "address"}
		],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [
			{"internalType": "address", "name": "owner", "type": "address"},
			{"internalType": "address", "name": "operator", "type": "address"}
		],
		"name": "isApprovedForAll",
		"outputs": [
			{"internalType": "bool", "name": "", "type": "bool"}
		],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [],
		"name": "name",
		"outputs": [
			{"internalType": "string", "name": "", "type": "string"}
		],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [],
		"name": "owner",
		"outputs": [
			{"internalType": "address", "name": "", "type": "address"}
		],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [
			{"internalType": "uint256", "name": "tokenId", "type": "uint256"}
		],
		"name": "ownerOf",
		"outputs": [
			{"internalType": "address", "name": "", "type": "address"}
		],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [],
		"name": "renounceOwnership",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [
			{"internalType": "address", "name": "to", "type": "address"},
			{"internalType": "string", "name": "uri", "type": "string"}
		],
		"name": "safeMint",
		"outputs": [
			{"internalType": "uint256", "name": "", "type": "uint256"}
		],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [
			{"internalType": "address", "name": "from", "type": "address"},
			{"internalType": "address", "name": "to", "type": "address"},
			{"internalType": "uint256", "name": "tokenId", "type": "uint256"}
		],
		"name": "safeTransferFrom",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [
			{"internalType": "address", "name": "from", "type": "address"},
			{"internalType": "address", "name": "to", "type": "address"},
			{"internalType": "uint256", "name": "tokenId", "type": "uint256"},
			{"internalType": "bytes", "name": "data", "type": "bytes"}
		],
		"name": "safeTransferFrom",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [
			{"internalType": "address", "name": "operator", "type": "address"},
			{"internalType": "bool", "name": "approved", "type": "bool"}
		],
		"name": "setApprovalForAll",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [
			{"internalType": "bytes4", "name": "interfaceId", "type": "bytes4"}
		],
		"name": "supportsInterface",
		"outputs": [
			{"internalType": "bool", "name": "", "type": "bool"}
		],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [],
		"name": "symbol",
		"outputs": [
			{"internalType": "string", "name": "", "type": "string"}
		],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [
			{"internalType": "uint256", "name": "tokenId", "type": "uint256"}
		],
		"name": "tokenURI",
		"outputs": [
			{"internalType": "string", "name": "", "type": "string"}
		],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [
			{"internalType": "address", "name": "from", "type": "address"},
			{"internalType": "address", "name": "to", "type": "address"},
			{"internalType": "uint256", "name": "tokenId", "type": "uint256"}
		],
		"name": "transferFrom",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [
			{"internalType": "address", "name": "newOwner", "type": "address"}
		],
		"name": "transferOwnership",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	}
]`

// MetaNodeConfig MetaNodeNFT配置
type MetaNodeConfig struct {
	ContractAddress string   // 合约地址
	OwnerPrivateKey string   // 所有者私钥
	RPCEndpoint     string   // RPC节点地址
	ChainID         int64    // 链ID
	GasLimit        uint64   // Gas限制
	GasPrice        *big.Int // Gas价格
}

// MintMetaNodeNFT 铸造MetaNodeNFT
func MintMetaNodeNFT(ctx context.Context, svcCtx *svc.ServerCtx, req *typesv1.MetaNodeMintRequest) (*typesv1.MetaNodeMintResult, error) {
	xzap.WithContext(ctx).Info("Starting MetaNodeNFT mint",
		zap.Int("chain_id", req.ChainID),
		zap.String("to_address", req.ToAddress),
		zap.String("token_uri", req.TokenURI),
	)

	// 获取配置
	config, err := getMetaNodeConfig(svcCtx, req.ChainID)
	if err != nil {
		xzap.WithContext(ctx).Error("Failed to get MetaNode config", zap.Error(err))
		return nil, errors.Wrap(err, "failed to get MetaNode config")
	}

	xzap.WithContext(ctx).Info("MetaNode config loaded",
		zap.String("contract_address", config.ContractAddress),
		zap.String("rpc_endpoint", config.RPCEndpoint),
		zap.Int64("chain_id", config.ChainID),
		zap.Uint64("gas_limit", config.GasLimit),
	)

	// 验证请求参数
	if err := validateMetaNodeMintRequest(req); err != nil {
		xzap.WithContext(ctx).Error("Invalid mint request", zap.Error(err))
		return nil, errors.Wrap(err, "invalid mint request")
	}

	// 连接到以太坊节点
	xzap.WithContext(ctx).Info("Connecting to Ethereum client", zap.String("rpc_endpoint", config.RPCEndpoint))
	client, err := ethclient.Dial(config.RPCEndpoint)
	if err != nil {
		xzap.WithContext(ctx).Error("Failed to connect to Ethereum client", zap.Error(err))
		return nil, errors.Wrap(err, "failed to connect to Ethereum client")
	}
	defer client.Close()

	// 测试连接
	chainID, err := client.ChainID(ctx)
	if err != nil {
		xzap.WithContext(ctx).Error("Failed to get chain ID from client", zap.Error(err))
		return nil, errors.Wrap(err, "failed to get chain ID")
	}
	xzap.WithContext(ctx).Info("Connected to Ethereum client",
		zap.String("chain_id", chainID.String()),
		zap.Int64("expected_chain_id", config.ChainID),
	)

	// 解析私钥
	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(config.OwnerPrivateKey, "0x"))
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse private key")
	}

	// 获取公钥和地址
	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, errors.New("error casting public key to ECDSA")
	}
	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)

	// 获取nonce
	nonce, err := client.PendingNonceAt(ctx, fromAddress)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get nonce")
	}

	// 获取Gas价格
	gasPrice := config.GasPrice
	if gasPrice == nil {
		gasPrice, err = client.SuggestGasPrice(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to suggest gas price")
		}
	}

	// 创建交易选项
	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, big.NewInt(config.ChainID))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create transactor")
	}
	auth.Nonce = big.NewInt(int64(nonce))
	auth.Value = big.NewInt(0)
	auth.GasLimit = config.GasLimit
	auth.GasPrice = gasPrice

	// 解析合约ABI
	contractABI, err := abi.JSON(strings.NewReader(MetaNodeNFTABI))
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse contract ABI")
	}

	// 创建合约实例
	contractAddress := common.HexToAddress(config.ContractAddress)
	contract := bind.NewBoundContract(contractAddress, contractABI, client, client, client)

	// 调用safeMint函数
	toAddress := common.HexToAddress(req.ToAddress)

	// 调用safeMint函数 - 这个函数会返回tokenId
	xzap.WithContext(ctx).Info("Calling safeMint function",
		zap.String("contract_address", config.ContractAddress),
		zap.String("to_address", req.ToAddress),
		zap.String("token_uri", req.TokenURI),
		zap.String("from_address", fromAddress.Hex()),
		zap.Uint64("nonce", nonce),
		zap.String("gas_price", gasPrice.String()),
		zap.Uint64("gas_limit", config.GasLimit),
	)

	tx, err := contract.Transact(auth, "safeMint", toAddress, req.TokenURI)
	if err != nil {
		xzap.WithContext(ctx).Error("Failed to call safeMint", zap.Error(err))
		return nil, errors.Wrap(err, "failed to call safeMint")
	}

	xzap.WithContext(ctx).Info("MetaNodeNFT mint transaction sent",
		zap.String("tx_hash", tx.Hash().Hex()),
		zap.String("to_address", req.ToAddress),
		zap.String("token_uri", req.TokenURI),
		zap.String("contract_address", config.ContractAddress),
	)

	// 等待交易确认（可选，用于获取tokenID）
	receipt, err := waitForTransaction(ctx, client, tx.Hash(), 30*time.Second)
	if err != nil {
		// 如果等待超时，仍然返回pending状态
		xzap.WithContext(ctx).Warn("transaction confirmation timeout", zap.Error(err))
		return &typesv1.MetaNodeMintResult{
			TxHash:   tx.Hash().Hex(),
			TokenID:  "", // 无法获取，需要后续查询
			Status:   "pending",
			GasPrice: gasPrice.String(),
		}, nil
	}

	// 解析事件获取TokenID
	tokenID, err := parseTokenIDFromReceipt(receipt, contractABI)
	if err != nil {
		xzap.WithContext(ctx).Warn("failed to parse tokenID from receipt", zap.Error(err))
		tokenID = "" // 设置为空，可以后续查询
	}

	// 铸造成功后，写入数据库
	if tokenID != "" && getTransactionStatus(receipt) == "confirmed" {
		if err := insertMintedItemToDB(ctx, svcCtx, req.ChainID, config.ContractAddress, tokenID, req.ToAddress, req.Name); err != nil {
			xzap.WithContext(ctx).Error("failed to insert minted item to database", zap.Error(err))
			// 数据库写入失败不影响铸造结果，但记录错误
		} else {
			xzap.WithContext(ctx).Info("successfully inserted minted item to database",
				zap.String("chain_id", fmt.Sprintf("%d", req.ChainID)),
				zap.String("collection_address", config.ContractAddress),
				zap.String("token_id", tokenID),
				zap.String("owner", req.ToAddress),
			)
		}
	}

	return &typesv1.MetaNodeMintResult{
		TxHash:      tx.Hash().Hex(),
		TokenID:     tokenID,
		Status:      getTransactionStatus(receipt),
		BlockNumber: receipt.BlockNumber.Int64(),
		GasUsed:     int64(receipt.GasUsed),
		GasPrice:    gasPrice.String(),
	}, nil
}

// BatchMintMetaNodeNFT 批量铸造MetaNodeNFT
func BatchMintMetaNodeNFT(ctx context.Context, svcCtx *svc.ServerCtx, req *typesv1.MetaNodeBatchMintRequest) (*typesv1.MetaNodeBatchMintResult, error) {
	// 获取配置
	_, err := getMetaNodeConfig(svcCtx, req.ChainID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get MetaNode config")
	}

	// 验证批量请求
	if len(req.Mints) == 0 {
		return nil, errors.New("no mint requests provided")
	}
	if len(req.Mints) > 50 { // 限制批量数量
		return nil, errors.New("too many mint requests, maximum 50 allowed")
	}

	var tokenIDs []string
	successCount := 0
	failedCount := 0

	// 逐个执行铸造（实际项目中可以考虑批量合约调用）
	for _, mintInfo := range req.Mints {
		mintReq := &typesv1.MetaNodeMintRequest{
			ChainID:     req.ChainID,
			ToAddress:   mintInfo.ToAddress,
			TokenURI:    mintInfo.TokenURI,
			Name:        mintInfo.Name,
			Description: mintInfo.Description,
		}

		result, err := MintMetaNodeNFT(ctx, svcCtx, mintReq)
		if err != nil {
			xzap.WithContext(ctx).Error("batch mint failed for item",
				zap.String("to_address", mintInfo.ToAddress),
				zap.Error(err))
			failedCount++
			tokenIDs = append(tokenIDs, "") // 占位符
			continue
		}

		successCount++
		tokenIDs = append(tokenIDs, result.TokenID)
	}

	return &typesv1.MetaNodeBatchMintResult{
		TxHash:       "", // 批量操作没有单一交易哈希
		TokenIDs:     tokenIDs,
		SuccessCount: successCount,
		FailedCount:  failedCount,
		Status:       getBatchStatus(successCount, failedCount),
	}, nil
}

// QueryMetaNodeNFT 查询MetaNodeNFT信息
func QueryMetaNodeNFT(ctx context.Context, svcCtx *svc.ServerCtx, req *typesv1.MetaNodeQueryRequest) (*typesv1.MetaNodeQueryResponse, error) {
	// 获取配置
	config, err := getMetaNodeConfig(svcCtx, req.ChainID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get MetaNode config")
	}

	// 连接到以太坊节点
	client, err := ethclient.Dial(config.RPCEndpoint)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to Ethereum client")
	}
	defer client.Close()

	// 解析合约ABI
	contractABI, err := abi.JSON(strings.NewReader(MetaNodeNFTABI))
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse contract ABI")
	}

	// 创建合约实例
	contractAddress := common.HexToAddress(config.ContractAddress)
	contract := bind.NewBoundContract(contractAddress, contractABI, client, client, client)

	var tokens []typesv1.MetaNodeTokenInfo

	if req.TokenID != "" {
		// 查询特定TokenID
		tokenInfo, err := queryTokenInfo(ctx, contract, req.TokenID)
		if err != nil {
			return nil, errors.Wrap(err, "failed to query token info")
		}
		tokens = append(tokens, *tokenInfo)
	} else {
		// 这里需要实现更复杂的查询逻辑，比如通过事件日志查询
		// 由于合约没有提供枚举接口，这里返回空结果
		// 实际项目中可以通过监听Transfer事件来建立索引
	}

	return &typesv1.MetaNodeQueryResponse{
		Tokens: tokens,
		Total:  int64(len(tokens)),
		Page:   req.Page,
		Size:   len(tokens),
	}, nil
}

// getMetaNodeConfig 获取MetaNodeNFT配置
func getMetaNodeConfig(svcCtx *svc.ServerCtx, chainID int) (*MetaNodeConfig, error) {
	// 从配置文件获取配置
	if svcCtx.C.MetaNode == nil {
		return nil, errors.New("MetaNode configuration not found in config file")
	}

	config := svcCtx.C.MetaNode
	chainIDStr := fmt.Sprintf("%d", chainID)

	// 获取合约地址
	contractAddr, exists := config.ContractAddresses[chainIDStr]
	if !exists || contractAddr == "" {
		return nil, fmt.Errorf("contract address not configured for chain ID: %d", chainID)
	}

	// 获取RPC端点
	rpcEndpoint, exists := config.RPCEndpoints[chainIDStr]
	if !exists || rpcEndpoint == "" {
		return nil, fmt.Errorf("RPC endpoint not configured for chain ID: %d", chainID)
	}

	// 检查私钥
	if config.OwnerPrivateKey == "" {
		return nil, errors.New("owner private key not configured")
	}

	// 解析Gas价格
	var gasPrice *big.Int
	if config.GasPrice != "" {
		var ok bool
		gasPrice, ok = new(big.Int).SetString(config.GasPrice, 10)
		if !ok {
			return nil, errors.New("invalid gas price format")
		}
	}

	// 设置默认Gas限制
	gasLimit := config.GasLimit
	if gasLimit == 0 {
		gasLimit = 300000
	}

	return &MetaNodeConfig{
		ContractAddress: contractAddr,
		OwnerPrivateKey: config.OwnerPrivateKey,
		RPCEndpoint:     rpcEndpoint,
		ChainID:         int64(chainID),
		GasLimit:        gasLimit,
		GasPrice:        gasPrice,
	}, nil
}

// validateMetaNodeMintRequest 验证铸造请求
func validateMetaNodeMintRequest(req *typesv1.MetaNodeMintRequest) error {
	if req.ToAddress == "" {
		return errors.New("to_address is required")
	}
	if !common.IsHexAddress(req.ToAddress) {
		return errors.New("invalid to_address format")
	}
	if req.TokenURI == "" {
		return errors.New("token_uri is required")
	}
	if req.ChainID <= 0 {
		return errors.New("invalid chain_id")
	}
	return nil
}

// waitForTransaction 等待交易确认
func waitForTransaction(ctx context.Context, client *ethclient.Client, txHash common.Hash, timeout time.Duration) (*types.Receipt, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			receipt, err := client.TransactionReceipt(ctx, txHash)
			if err == nil {
				return receipt, nil
			}
			// 继续等待
		}
	}
}

// parseTokenIDFromReceipt 从交易收据解析TokenID
func parseTokenIDFromReceipt(receipt *types.Receipt, contractABI abi.ABI) (string, error) {
	// 解析Transfer事件来获取TokenID
	transferEventSignature := crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)"))

	for _, log := range receipt.Logs {
		if len(log.Topics) > 0 && log.Topics[0] == transferEventSignature {
			// Transfer事件结构: Transfer(address indexed from, address indexed to, uint256 indexed tokenId)
			// Topics[0]: 事件签名
			// Topics[1]: from地址 (0x0000...0000 表示mint)
			// Topics[2]: to地址
			// Topics[3]: tokenId

			if len(log.Topics) >= 4 {
				// 检查是否是mint操作 (from地址为0)
				fromAddress := common.HexToAddress(log.Topics[1].Hex())
				if fromAddress == (common.Address{}) {
					// 这是一个mint操作，获取tokenId
					tokenID := log.Topics[3].Big()
					return tokenID.String(), nil
				}
			}
		}
	}

	return "", errors.New("mint Transfer event not found in transaction receipt")
}

// getTransactionStatus 获取交易状态
func getTransactionStatus(receipt *types.Receipt) string {
	if receipt.Status == 1 {
		return "confirmed"
	}
	return "failed"
}

// getBatchStatus 获取批量操作状态
func getBatchStatus(successCount, failedCount int) string {
	if failedCount == 0 {
		return "all_success"
	} else if successCount == 0 {
		return "all_failed"
	}
	return "partial_success"
}

// queryTokenInfo 查询Token信息
func queryTokenInfo(ctx context.Context, contract *bind.BoundContract, tokenIDStr string) (*typesv1.MetaNodeTokenInfo, error) {
	tokenID, ok := new(big.Int).SetString(tokenIDStr, 10)
	if !ok {
		return nil, errors.New("invalid token ID")
	}

	// 查询tokenURI
	var tokenURIResult []interface{}
	err := contract.Call(&bind.CallOpts{}, &tokenURIResult, "tokenURI", tokenID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tokenURI")
	}

	var tokenURI string
	if len(tokenURIResult) > 0 {
		if uri, ok := tokenURIResult[0].(string); ok {
			tokenURI = uri
		}
	}

	// 查询owner
	var ownerResult []interface{}
	err = contract.Call(&bind.CallOpts{}, &ownerResult, "ownerOf", tokenID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get owner")
	}

	var owner common.Address
	if len(ownerResult) > 0 {
		if addr, ok := ownerResult[0].(common.Address); ok {
			owner = addr
		}
	}

	return &typesv1.MetaNodeTokenInfo{
		TokenID:  tokenIDStr,
		Owner:    owner.Hex(),
		TokenURI: tokenURI,
		MintedAt: time.Now().Unix(), // 这里需要从区块链获取实际时间
	}, nil
}

// insertMintedItemToDB 将铸造的NFT信息插入到数据库中
func insertMintedItemToDB(ctx context.Context, svcCtx *svc.ServerCtx, chainID int, collectionAddress, tokenID, owner, name string) error {
	// 构造链名称
	chainName := getChainName(chainID)

	// 构造要插入的数据
	item := map[string]interface{}{
		"chain_id":           chainID,
		"collection_address": collectionAddress,
		"token_id":           tokenID,
		"owner":              owner,
		"name":               name,
		"is_opensea_banned":  false, // 默认设置为false
		"create_time":        time.Now().Unix(),
		"update_time":        time.Now().Unix(),
	}

	// 获取表名并插入数据
	tableName := multi.ItemTableName(chainName)
	if err := svcCtx.Dao.DB.WithContext(ctx).Table(tableName).Create(&item).Error; err != nil {
		return errors.Wrap(err, "failed to insert minted item to database")
	}

	return nil
}

// getChainName 根据chainID获取链名称
func getChainName(chainID int) string {
	// 根据chainID映射到链名称
	// 这里需要根据实际的链ID映射关系来实现
	switch chainID {
	case 1:
		return "ethereum"
	case 11155111:
		return "sepolia"
	case 56:
		return "bsc"
	case 137:
		return "polygon"
	case 43114:
		return "avalanche"
	default:
		return fmt.Sprintf("chain_%d", chainID) // 默认格式
	}
}
