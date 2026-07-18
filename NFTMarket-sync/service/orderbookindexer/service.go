package orderbookindexer

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/ProjectsTask/EasySwapBase/chain/chainclient"
	"github.com/ProjectsTask/EasySwapBase/chain/types"
	"github.com/ProjectsTask/EasySwapBase/logger/xzap"
	"github.com/ProjectsTask/EasySwapBase/ordermanager"
	"github.com/ProjectsTask/EasySwapBase/stores/gdb"
	"github.com/ProjectsTask/EasySwapBase/stores/gdb/orderbookmodel/base"
	"github.com/ProjectsTask/EasySwapBase/stores/gdb/orderbookmodel/multi"
	"github.com/ProjectsTask/EasySwapBase/stores/xkv"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethereumTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/threading"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/ProjectsTask/EasySwapSync/service/comm"
	"github.com/ProjectsTask/EasySwapSync/service/config"
)

const (
	EventIndexType  = 6
	SleepInterval   = 10   // in seconds
	SyncBlockPeriod = 1000 // 提高同步效率，每次处理8个区块
	MinSyncBlocks   = 500  // 每轮最少同步区块数（不足则等待累积）
	LogMakeTopic    = "0xfc37f2ff950f95913eb7182357ba3c14df60ef354bc7d6ab1ba2815f249fffe6"
	// keccak256("LogCancel(OrderKey orderKey)")
	LogCancelTopic = "0x0ac8bb53fac566d7afc05d8b4df11d7690a7b27bdc40b54e4060f9b21fb849bd"
	// keccak256("LogMatch(OrderKey makeOrderKey, OrderKey takeOrderKey, Order makeOrder, Order takeOrder, uint128 fillPrice)")
	LogMatchTopic = "0xf629aecab94607bc43ce4aebd564bf6e61c7327226a797b002de724b9944b20e"
	// keccak256("LogSkipOrder(OrderKey orderKey, uint64 salt)")
	ERC721ApprovalTopic = "0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925"
	contractAbi         = `[{"inputs":[],"name":"CannotFindNextEmptyKey","type":"error"},{"inputs":[],"name":"CannotFindPrevEmptyKey","type":"error"},{"inputs":[{"internalType":"OrderKey","name":"orderKey","type":"bytes32"}],"name":"CannotInsertDuplicateOrder","type":"error"},{"inputs":[],"name":"CannotInsertEmptyKey","type":"error"},{"inputs":[],"name":"CannotInsertExistingKey","type":"error"},{"inputs":[],"name":"CannotRemoveEmptyKey","type":"error"},{"inputs":[],"name":"CannotRemoveMissingKey","type":"error"},{"inputs":[],"name":"EnforcedPause","type":"error"},{"inputs":[],"name":"ExpectedPause","type":"error"},{"inputs":[],"name":"InvalidInitialization","type":"error"},{"inputs":[],"name":"NotInitializing","type":"error"},{"inputs":[{"internalType":"address","name":"owner","type":"address"}],"name":"OwnableInvalidOwner","type":"error"},{"inputs":[{"internalType":"address","name":"account","type":"address"}],"name":"OwnableUnauthorizedAccount","type":"error"},{"inputs":[],"name":"ReentrancyGuardReentrantCall","type":"error"},{"anonymous":false,"inputs":[{"indexed":false,"internalType":"uint256","name":"offset","type":"uint256"},{"indexed":false,"internalType":"bytes","name":"msg","type":"bytes"}],"name":"BatchMatchInnerError","type":"event"},{"anonymous":false,"inputs":[],"name":"EIP712DomainChanged","type":"event"},{"anonymous":false,"inputs":[{"indexed":false,"internalType":"uint64","name":"version","type":"uint64"}],"name":"Initialized","type":"event"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"OrderKey","name":"orderKey","type":"bytes32"},{"indexed":true,"internalType":"address","name":"maker","type":"address"}],"name":"LogCancel","type":"event"},{"anonymous":false,"inputs":[{"indexed":false,"internalType":"OrderKey","name":"orderKey","type":"bytes32"},{"indexed":true,"internalType":"enumLibOrder.Side","name":"side","type":"uint8"},{"indexed":true,"internalType":"enumLibOrder.SaleKind","name":"saleKind","type":"uint8"},{"indexed":true,"internalType":"address","name":"maker","type":"address"},{"components":[{"internalType":"uint256","name":"tokenId","type":"uint256"},{"internalType":"address","name":"collection","type":"address"},{"internalType":"uint96","name":"amount","type":"uint96"}],"indexed":false,"internalType":"structLibOrder.Asset","name":"nft","type":"tuple"},{"indexed":false,"internalType":"Price","name":"price","type":"uint128"},{"indexed":false,"internalType":"uint64","name":"expiry","type":"uint64"},{"indexed":false,"internalType":"uint64","name":"salt","type":"uint64"}],"name":"LogMake","type":"event"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"OrderKey","name":"makeOrderKey","type":"bytes32"},{"indexed":true,"internalType":"OrderKey","name":"takeOrderKey","type":"bytes32"},{"components":[{"internalType":"enumLibOrder.Side","name":"side","type":"uint8"},{"internalType":"enumLibOrder.SaleKind","name":"saleKind","type":"uint8"},{"internalType":"address","name":"maker","type":"address"},{"components":[{"internalType":"uint256","name":"tokenId","type":"uint256"},{"internalType":"address","name":"collection","type":"address"},{"internalType":"uint96","name":"amount","type":"uint96"}],"internalType":"structLibOrder.Asset","name":"nft","type":"tuple"},{"internalType":"Price","name":"price","type":"uint128"},{"internalType":"uint64","name":"expiry","type":"uint64"},{"internalType":"uint64","name":"salt","type":"uint64"}],"indexed":false,"internalType":"structLibOrder.Order","name":"makeOrder","type":"tuple"},{"components":[{"internalType":"enumLibOrder.Side","name":"side","type":"uint8"},{"internalType":"enumLibOrder.SaleKind","name":"saleKind","type":"uint8"},{"internalType":"address","name":"maker","type":"address"},{"components":[{"internalType":"uint256","name":"tokenId","type":"uint256"},{"internalType":"address","name":"collection","type":"address"},{"internalType":"uint96","name":"amount","type":"uint96"}],"internalType":"structLibOrder.Asset","name":"nft","type":"tuple"},{"internalType":"Price","name":"price","type":"uint128"},{"internalType":"uint64","name":"expiry","type":"uint64"},{"internalType":"uint64","name":"salt","type":"uint64"}],"indexed":false,"internalType":"structLibOrder.Order","name":"takeOrder","type":"tuple"},{"indexed":false,"internalType":"uint128","name":"fillPrice","type":"uint128"}],"name":"LogMatch","type":"event"},{"anonymous":false,"inputs":[{"indexed":false,"internalType":"OrderKey","name":"orderKey","type":"bytes32"},{"indexed":false,"internalType":"uint64","name":"salt","type":"uint64"}],"name":"LogSkipOrder","type":"event"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"uint128","name":"newProtocolShare","type":"uint128"}],"name":"LogUpdatedProtocolShare","type":"event"},{"anonymous":false,"inputs":[{"indexed":false,"internalType":"address","name":"recipient","type":"address"},{"indexed":false,"internalType":"uint256","name":"amount","type":"uint256"}],"name":"LogWithdrawETH","type":"event"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"address","name":"previousOwner","type":"address"},{"indexed":true,"internalType":"address","name":"newOwner","type":"address"}],"name":"OwnershipTransferred","type":"event"},{"anonymous":false,"inputs":[{"indexed":false,"internalType":"address","name":"account","type":"address"}],"name":"Paused","type":"event"},{"anonymous":false,"inputs":[{"indexed":false,"internalType":"address","name":"account","type":"address"}],"name":"Unpaused","type":"event"},{"inputs":[{"internalType":"OrderKey[]","name":"orderKeys","type":"bytes32[]"}],"name":"cancelOrders","outputs":[{"internalType":"bool[]","name":"successes","type":"bool[]"}],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"components":[{"internalType":"OrderKey","name":"oldOrderKey","type":"bytes32"},{"components":[{"internalType":"enumLibOrder.Side","name":"side","type":"uint8"},{"internalType":"enumLibOrder.SaleKind","name":"saleKind","type":"uint8"},{"internalType":"address","name":"maker","type":"address"},{"components":[{"internalType":"uint256","name":"tokenId","type":"uint256"},{"internalType":"address","name":"collection","type":"address"},{"internalType":"uint96","name":"amount","type":"uint96"}],"internalType":"structLibOrder.Asset","name":"nft","type":"tuple"},{"internalType":"Price","name":"price","type":"uint128"},{"internalType":"uint64","name":"expiry","type":"uint64"},{"internalType":"uint64","name":"salt","type":"uint64"}],"internalType":"structLibOrder.Order","name":"newOrder","type":"tuple"}],"internalType":"structLibOrder.EditDetail[]","name":"editDetails","type":"tuple[]"}],"name":"editOrders","outputs":[{"internalType":"OrderKey[]","name":"newOrderKeys","type":"bytes32[]"}],"stateMutability":"payable","type":"function"},{"inputs":[],"name":"eip712Domain","outputs":[{"internalType":"bytes1","name":"fields","type":"bytes1"},{"internalType":"string","name":"name","type":"string"},{"internalType":"string","name":"version","type":"string"},{"internalType":"uint256","name":"chainId","type":"uint256"},{"internalType":"address","name":"verifyingContract","type":"address"},{"internalType":"bytes32","name":"salt","type":"bytes32"},{"internalType":"uint256[]","name":"extensions","type":"uint256[]"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"OrderKey","name":"","type":"bytes32"}],"name":"filledAmount","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"collection","type":"address"},{"internalType":"uint256","name":"tokenId","type":"uint256"},{"internalType":"enumLibOrder.Side","name":"side","type":"uint8"},{"internalType":"enumLibOrder.SaleKind","name":"saleKind","type":"uint8"}],"name":"getBestOrder","outputs":[{"components":[{"internalType":"enumLibOrder.Side","name":"side","type":"uint8"},{"internalType":"enumLibOrder.SaleKind","name":"saleKind","type":"uint8"},{"internalType":"address","name":"maker","type":"address"},{"components":[{"internalType":"uint256","name":"tokenId","type":"uint256"},{"internalType":"address","name":"collection","type":"address"},{"internalType":"uint96","name":"amount","type":"uint96"}],"internalType":"structLibOrder.Asset","name":"nft","type":"tuple"},{"internalType":"Price","name":"price","type":"uint128"},{"internalType":"uint64","name":"expiry","type":"uint64"},{"internalType":"uint64","name":"salt","type":"uint64"}],"internalType":"structLibOrder.Order","name":"orderResult","type":"tuple"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"collection","type":"address"},{"internalType":"enumLibOrder.Side","name":"side","type":"uint8"}],"name":"getBestPrice","outputs":[{"internalType":"Price","name":"price","type":"uint128"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"collection","type":"address"},{"internalType":"enumLibOrder.Side","name":"side","type":"uint8"},{"internalType":"Price","name":"price","type":"uint128"}],"name":"getNextBestPrice","outputs":[{"internalType":"Price","name":"nextBestPrice","type":"uint128"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"collection","type":"address"},{"internalType":"uint256","name":"tokenId","type":"uint256"},{"internalType":"enumLibOrder.Side","name":"side","type":"uint8"},{"internalType":"enumLibOrder.SaleKind","name":"saleKind","type":"uint8"},{"internalType":"uint256","name":"count","type":"uint256"},{"internalType":"Price","name":"price","type":"uint128"},{"internalType":"OrderKey","name":"firstOrderKey","type":"bytes32"}],"name":"getOrders","outputs":[{"components":[{"internalType":"enumLibOrder.Side","name":"side","type":"uint8"},{"internalType":"enumLibOrder.SaleKind","name":"saleKind","type":"uint8"},{"internalType":"address","name":"maker","type":"address"},{"components":[{"internalType":"uint256","name":"tokenId","type":"uint256"},{"internalType":"address","name":"collection","type":"address"},{"internalType":"uint96","name":"amount","type":"uint96"}],"internalType":"structLibOrder.Asset","name":"nft","type":"tuple"},{"internalType":"Price","name":"price","type":"uint128"},{"internalType":"uint64","name":"expiry","type":"uint64"},{"internalType":"uint64","name":"salt","type":"uint64"}],"internalType":"structLibOrder.Order[]","name":"resultOrders","type":"tuple[]"},{"internalType":"OrderKey","name":"nextOrderKey","type":"bytes32"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"uint128","name":"newProtocolShare","type":"uint128"},{"internalType":"address","name":"newVault","type":"address"},{"internalType":"string","name":"EIP712Name","type":"string"},{"internalType":"string","name":"EIP712Version","type":"string"}],"name":"initialize","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"components":[{"internalType":"enumLibOrder.Side","name":"side","type":"uint8"},{"internalType":"enumLibOrder.SaleKind","name":"saleKind","type":"uint8"},{"internalType":"address","name":"maker","type":"address"},{"components":[{"internalType":"uint256","name":"tokenId","type":"uint256"},{"internalType":"address","name":"collection","type":"address"},{"internalType":"uint96","name":"amount","type":"uint96"}],"internalType":"structLibOrder.Asset","name":"nft","type":"tuple"},{"internalType":"Price","name":"price","type":"uint128"},{"internalType":"uint64","name":"expiry","type":"uint64"},{"internalType":"uint64","name":"salt","type":"uint64"}],"internalType":"structLibOrder.Order[]","name":"newOrders","type":"tuple[]"}],"name":"makeOrders","outputs":[{"internalType":"OrderKey[]","name":"newOrderKeys","type":"bytes32[]"}],"stateMutability":"payable","type":"function"},{"inputs":[{"components":[{"internalType":"enumLibOrder.Side","name":"side","type":"uint8"},{"internalType":"enumLibOrder.SaleKind","name":"saleKind","type":"uint8"},{"internalType":"address","name":"maker","type":"address"},{"components":[{"internalType":"uint256","name":"tokenId","type":"uint256"},{"internalType":"address","name":"collection","type":"address"},{"internalType":"uint96","name":"amount","type":"uint96"}],"internalType":"structLibOrder.Asset","name":"nft","type":"tuple"},{"internalType":"Price","name":"price","type":"uint128"},{"internalType":"uint64","name":"expiry","type":"uint64"},{"internalType":"uint64","name":"salt","type":"uint64"}],"internalType":"structLibOrder.Order","name":"sellOrder","type":"tuple"},{"components":[{"internalType":"enumLibOrder.Side","name":"side","type":"uint8"},{"internalType":"enumLibOrder.SaleKind","name":"saleKind","type":"uint8"},{"internalType":"address","name":"maker","type":"address"},{"components":[{"internalType":"uint256","name":"tokenId","type":"uint256"},{"internalType":"address","name":"collection","type":"address"},{"internalType":"uint96","name":"amount","type":"uint96"}],"internalType":"structLibOrder.Asset","name":"nft","type":"tuple"},{"internalType":"Price","name":"price","type":"uint128"},{"internalType":"uint64","name":"expiry","type":"uint64"},{"internalType":"uint64","name":"salt","type":"uint64"}],"internalType":"structLibOrder.Order","name":"buyOrder","type":"tuple"}],"name":"matchOrder","outputs":[],"stateMutability":"payable","type":"function"},{"inputs":[{"components":[{"internalType":"enumLibOrder.Side","name":"side","type":"uint8"},{"internalType":"enumLibOrder.SaleKind","name":"saleKind","type":"uint8"},{"internalType":"address","name":"maker","type":"address"},{"components":[{"internalType":"uint256","name":"tokenId","type":"uint256"},{"internalType":"address","name":"collection","type":"address"},{"internalType":"uint96","name":"amount","type":"uint96"}],"internalType":"structLibOrder.Asset","name":"nft","type":"tuple"},{"internalType":"Price","name":"price","type":"uint128"},{"internalType":"uint64","name":"expiry","type":"uint64"},{"internalType":"uint64","name":"salt","type":"uint64"}],"internalType":"structLibOrder.Order","name":"sellOrder","type":"tuple"},{"components":[{"internalType":"enumLibOrder.Side","name":"side","type":"uint8"},{"internalType":"enumLibOrder.SaleKind","name":"saleKind","type":"uint8"},{"internalType":"address","name":"maker","type":"address"},{"components":[{"internalType":"uint256","name":"tokenId","type":"uint256"},{"internalType":"address","name":"collection","type":"address"},{"internalType":"uint96","name":"amount","type":"uint96"}],"internalType":"structLibOrder.Asset","name":"nft","type":"tuple"},{"internalType":"Price","name":"price","type":"uint128"},{"internalType":"uint64","name":"expiry","type":"uint64"},{"internalType":"uint64","name":"salt","type":"uint64"}],"internalType":"structLibOrder.Order","name":"buyOrder","type":"tuple"},{"internalType":"uint256","name":"msgValue","type":"uint256"}],"name":"matchOrderWithoutPayback","outputs":[{"internalType":"uint128","name":"costValue","type":"uint128"}],"stateMutability":"payable","type":"function"},{"inputs":[{"components":[{"components":[{"internalType":"enumLibOrder.Side","name":"side","type":"uint8"},{"internalType":"enumLibOrder.SaleKind","name":"saleKind","type":"uint8"},{"internalType":"address","name":"maker","type":"address"},{"components":[{"internalType":"uint256","name":"tokenId","type":"uint256"},{"internalType":"address","name":"collection","type":"address"},{"internalType":"uint96","name":"amount","type":"uint96"}],"internalType":"structLibOrder.Asset","name":"nft","type":"tuple"},{"internalType":"Price","name":"price","type":"uint128"},{"internalType":"uint64","name":"expiry","type":"uint64"},{"internalType":"uint64","name":"salt","type":"uint64"}],"internalType":"structLibOrder.Order","name":"sellOrder","type":"tuple"},{"components":[{"internalType":"enumLibOrder.Side","name":"side","type":"uint8"},{"internalType":"enumLibOrder.SaleKind","name":"saleKind","type":"uint8"},{"internalType":"address","name":"maker","type":"address"},{"components":[{"internalType":"uint256","name":"tokenId","type":"uint256"},{"internalType":"address","name":"collection","type":"address"},{"internalType":"uint96","name":"amount","type":"uint96"}],"internalType":"structLibOrder.Asset","name":"nft","type":"tuple"},{"internalType":"Price","name":"price","type":"uint128"},{"internalType":"uint64","name":"expiry","type":"uint64"},{"internalType":"uint64","name":"salt","type":"uint64"}],"internalType":"structLibOrder.Order","name":"buyOrder","type":"tuple"}],"internalType":"structLibOrder.MatchDetail[]","name":"matchDetails","type":"tuple[]"}],"name":"matchOrders","outputs":[{"internalType":"bool[]","name":"successes","type":"bool[]"}],"stateMutability":"payable","type":"function"},{"inputs":[{"internalType":"address","name":"","type":"address"},{"internalType":"enumLibOrder.Side","name":"","type":"uint8"},{"internalType":"Price","name":"","type":"uint128"}],"name":"orderQueues","outputs":[{"internalType":"OrderKey","name":"head","type":"bytes32"},{"internalType":"OrderKey","name":"tail","type":"bytes32"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"OrderKey","name":"","type":"bytes32"}],"name":"orders","outputs":[{"components":[{"internalType":"enumLibOrder.Side","name":"side","type":"uint8"},{"internalType":"enumLibOrder.SaleKind","name":"saleKind","type":"uint8"},{"internalType":"address","name":"maker","type":"address"},{"components":[{"internalType":"uint256","name":"tokenId","type":"uint256"},{"internalType":"address","name":"collection","type":"address"},{"internalType":"uint96","name":"amount","type":"uint96"}],"internalType":"structLibOrder.Asset","name":"nft","type":"tuple"},{"internalType":"Price","name":"price","type":"uint128"},{"internalType":"uint64","name":"expiry","type":"uint64"},{"internalType":"uint64","name":"salt","type":"uint64"}],"internalType":"structLibOrder.Order","name":"order","type":"tuple"},{"internalType":"OrderKey","name":"next","type":"bytes32"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"owner","outputs":[{"internalType":"address","name":"","type":"address"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"pause","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[],"name":"paused","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"","type":"address"},{"internalType":"enumLibOrder.Side","name":"","type":"uint8"}],"name":"priceTrees","outputs":[{"internalType":"Price","name":"root","type":"uint128"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"protocolShare","outputs":[{"internalType":"uint128","name":"","type":"uint128"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"renounceOwnership","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"uint128","name":"newProtocolShare","type":"uint128"}],"name":"setProtocolShare","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"newVault","type":"address"}],"name":"setVault","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"newOwner","type":"address"}],"name":"transferOwnership","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[],"name":"unpause","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"recipient","type":"address"},{"internalType":"uint256","name":"amount","type":"uint256"}],"name":"withdrawETH","outputs":[],"stateMutability":"nonpayable","type":"function"},{"stateMutability":"payable","type":"receive"}]`
	FixForCollection    = 0
	FixForItem          = 1
	List                = 0
	Bid                 = 1

	HexPrefix   = "0x"
	ZeroAddress = "0x0000000000000000000000000000000000000000"
)

type Order struct {
	Side     uint8
	SaleKind uint8
	Maker    common.Address
	Nft      struct {
		TokenId        *big.Int
		CollectionAddr common.Address
		Amount         *big.Int
	}
	Price  *big.Int
	Expiry uint64
	Salt   uint64
}

type Service struct {
	ctx          context.Context
	cfg          *config.Config
	db           *gorm.DB
	kv           *xkv.Store
	orderManager *ordermanager.OrderManager
	chainClient  chainclient.ChainClient
	chainId      int64
	chain        string
	parsedAbi    abi.ABI
	vaultAddress string
}

var MultiChainMaxBlockDifference = map[string]uint64{
	"eth":      8,
	"optimism": 8,

	"base":    8,
	"sepolia": 8, // Sepolia 测试网可能需要更多确认

}

func New(ctx context.Context, cfg *config.Config, db *gorm.DB, xkv *xkv.Store, chainClient chainclient.ChainClient, chainId int64, chain string, orderManager *ordermanager.OrderManager) *Service {

	parsedAbi, _ := abi.JSON(strings.NewReader(contractAbi)) // 通过ABI实例化
	return &Service{
		ctx:          ctx,
		cfg:          cfg,
		db:           db,
		kv:           xkv,
		chainClient:  chainClient,
		orderManager: orderManager,
		chain:        chain,
		chainId:      chainId,
		parsedAbi:    parsedAbi,
		vaultAddress: cfg.ContractCfg.VaultAddress,
	}
}

func (s *Service) Start() {
	threading.GoSafe(s.SyncOrderBookEventLoop)
	threading.GoSafe(s.UpKeepingCollectionFloorChangeLoop)
}

func (s *Service) SyncOrderBookEventLoop() {
	var indexedStatus base.IndexedStatus
	if err := s.db.WithContext(s.ctx).Table(base.IndexedStatusTableName()).
		Where("chain_id = ? and index_type = ?", s.chainId, EventIndexType).
		First(&indexedStatus).Error; err != nil {
		xzap.WithContext(s.ctx).Error("failed on get listing index status",
			zap.Error(err))
		return
	}

	lastSyncBlock := uint64(indexedStatus.LastIndexedBlock)
	for {
		select {
		case <-s.ctx.Done():
			xzap.WithContext(s.ctx).Info("SyncOrderBookEventLoop stopped due to context cancellation")
			return
		default:
		}

		currentBlockNum, err := s.chainClient.BlockNumber() // 以轮询的方式获取当前区块高度
		if err != nil {
			xzap.WithContext(s.ctx).Error("failed on get current block number", zap.Error(err))
			time.Sleep(SleepInterval * time.Second)
			continue
		}

		safeEndBlock := currentBlockNum - MultiChainMaxBlockDifference[s.chain]
		if lastSyncBlock > safeEndBlock { // 如果上次同步的区块高度大于当前区块高度，等待一段时间后再次轮询
			time.Sleep(SleepInterval * time.Second)
			continue
		}

		startBlock := lastSyncBlock
		availableBlocks := safeEndBlock - lastSyncBlock + 1
		var endBlock uint64
		// 自适应策略：
		// 1) 落后较多时按大批量（SyncBlockPeriod）追赶；
		// 2) 接近链头时允许小批量实时同步，避免“数据长期不入库”。
		if availableBlocks >= MinSyncBlocks {
			endBlock = startBlock + SyncBlockPeriod
			if endBlock > safeEndBlock {
				endBlock = safeEndBlock
			}
		} else {
			endBlock = safeEndBlock
		}

		query := types.FilterQuery{
			FromBlock: new(big.Int).SetUint64(startBlock),
			ToBlock:   new(big.Int).SetUint64(endBlock),
			Addresses: []string{s.cfg.ContractCfg.DexAddress},
		}

		logs, err := s.chainClient.FilterLogs(s.ctx, query) //同时获取多个（SyncBlockPeriod）区块的日志
		if err != nil {
			xzap.WithContext(s.ctx).Error("failed on get log",
				zap.Error(err),
				zap.Uint64("start_block", startBlock),
				zap.Uint64("end_block", endBlock),
				zap.Uint64("block_range", endBlock-startBlock+1))

			// 如果是因为请求范围太大导致的错误，尝试减少批量大小
			if endBlock-startBlock > 1 {
				xzap.WithContext(s.ctx).Warn("reducing block range due to RPC error",
					zap.Uint64("original_range", endBlock-startBlock+1),
					zap.Uint64("new_range", 1))
				// 只处理单个区块
				endBlock = startBlock
				query.ToBlock = new(big.Int).SetUint64(endBlock)

				// 重试单个区块
				logs, err = s.chainClient.FilterLogs(s.ctx, query)
				if err != nil {
					xzap.WithContext(s.ctx).Error("failed on get log even with single block",
						zap.Error(err),
						zap.Uint64("block", startBlock))
					time.Sleep(SleepInterval * time.Second)
					continue
				}
			} else {
				time.Sleep(SleepInterval * time.Second)
				continue
			}
		}

		for _, log := range logs { // 遍历日志，根据不同的topic处理不同的事件
			ethLog := log.(ethereumTypes.Log)
			switch ethLog.Topics[0].String() {
			case LogMakeTopic:
				s.handleMakeEvent(ethLog)
			case LogCancelTopic:
				s.handleCancelEvent(ethLog)
			case LogMatchTopic:
				s.handleMatchEvent(ethLog)
			case ERC721ApprovalTopic:
				s.handleApprovalEvent(ethLog)
			default:
			}
		}

		lastSyncBlock = endBlock + 1 // 更新最后同步的区块高度
		if err := s.db.WithContext(s.ctx).Table(base.IndexedStatusTableName()).
			Where("chain_id = ? and index_type = ?", s.chainId, EventIndexType).
			Update("last_indexed_block", lastSyncBlock).Error; err != nil {
			xzap.WithContext(s.ctx).Error("failed on update orderbook event sync block number",
				zap.Error(err))
			return
		}

		xzap.WithContext(s.ctx).Info("sync orderbook event ...",
			zap.Uint64("start_block", startBlock),
			zap.Uint64("end_block", endBlock))
	}
}

// 处理挂单事件
func (s *Service) handleMakeEvent(log ethereumTypes.Log) {
	// 添加分叉检查
	if err := s.checkAndHandleFork(log.BlockNumber, log.TxHash.String()); err != nil {
		xzap.WithContext(s.ctx).Error("failed to handle fork for make event",
			zap.Error(err),
			zap.Uint64("block_number", log.BlockNumber),
			zap.String("tx_hash", log.TxHash.String()))
		return
	}

	var event struct {
		OrderKey [32]byte
		Nft      struct {
			TokenId        *big.Int
			CollectionAddr common.Address
			Amount         *big.Int
		}
		Price  *big.Int
		Expiry uint64
		Salt   uint64
	}

	// Unpack data
	err := s.parsedAbi.UnpackIntoInterface(&event, "LogMake", log.Data) // 通过ABI解析日志数据
	if err != nil {
		xzap.WithContext(s.ctx).Error("Error unpacking LogMake event:", zap.Error(err))
		return
	}
	// Extract indexed fields from topics
	side := uint8(new(big.Int).SetBytes(log.Topics[1].Bytes()).Uint64())
	saleKind := uint8(new(big.Int).SetBytes(log.Topics[2].Bytes()).Uint64())
	maker := common.BytesToAddress(log.Topics[3].Bytes())

	var orderType int64
	if side == Bid { // 买单
		if saleKind == FixForCollection { // 针对集合的买单
			orderType = multi.CollectionBidOrder
		} else { // 针对某个具体NFT的买单
			orderType = multi.ItemBidOrder
		}
	} else { // 卖单
		orderType = multi.ListingOrder
	}
	newOrder := multi.Order{
		CollectionAddress: event.Nft.CollectionAddr.String(),
		MarketplaceId:     multi.MarketOrderBook,
		TokenId:           event.Nft.TokenId.String(),
		OrderID:           HexPrefix + hex.EncodeToString(event.OrderKey[:]),
		OrderStatus:       multi.OrderStatusActive,
		EventTime:         time.Now().Unix(),
		ExpireTime:        int64(event.Expiry),
		CurrencyAddress:   s.cfg.ContractCfg.EthAddress,
		Price:             decimal.NewFromBigInt(event.Price, 0),
		Maker:             maker.String(),
		Taker:             ZeroAddress,
		QuantityRemaining: event.Nft.Amount.Int64(),
		Size:              event.Nft.Amount.Int64(),
		OrderType:         orderType,
		Salt:              int64(event.Salt),
	}
	if err := s.db.WithContext(s.ctx).Table(multi.OrderTableName(s.chain)).Clauses(clause.OnConflict{
		DoNothing: true,
	}).Create(&newOrder).Error; err != nil { // 将订单信息存入数据库
		xzap.WithContext(s.ctx).Error("failed on create order",
			zap.Error(err))
	}

	newItem := multi.Item{
		CollectionAddress: event.Nft.CollectionAddr.String(),
		TokenId:           event.Nft.TokenId.String(),
		Owner:             maker.String(),
		Supply:            event.Nft.Amount.Int64(),
		ListPrice:         decimal.NewFromBigInt(event.Price, 0),
		ListTime:          time.Now().Unix(),
		UpdateTime:        time.Now().Unix(),
	}
	// 将 NFT 写入数据库
	if err = s.db.WithContext(s.ctx).Table(multi.ItemTableName(s.chain)).Clauses(clause.OnConflict{
		DoNothing: true,
	}).Create(&newItem).Error; err != nil {
		xzap.WithContext(s.ctx).Error("failed on create item",
			zap.Error(err))
	}

	// 写入 item_external 元数据表
	s.createItemExternal(event.Nft.CollectionAddr.String(), event.Nft.TokenId.String())

	// 业务上，挂卖单后必须优先维护 item/collection。
	// 放在这里避免受后续 blockTime/活动写入失败影响，保证主数据先落库。
	if side == List {
		s.syncItemOnListing(
			event.Nft.CollectionAddr.String(),
			event.Nft.TokenId.String(),
			maker.String(),
			decimal.NewFromBigInt(event.Price, 0),
		)
		s.maintainCollectionAndItem(
			event.Nft.CollectionAddr.String(),
			event.Nft.TokenId.String(),
			decimal.NewFromBigInt(event.Price, 0),
		)
	}

	blockTime, err := s.chainClient.BlockTimeByNumber(s.ctx, big.NewInt(int64(log.BlockNumber)))
	if err != nil {
		xzap.WithContext(s.ctx).Error("failed to get block time", zap.Error(err))
		return
	}
	var activityType int
	if side == Bid {
		if saleKind == FixForCollection {
			activityType = multi.CollectionBid
		} else {
			activityType = multi.ItemBid
		}
	} else {
		activityType = multi.Listing
	}
	newActivity := multi.Activity{ // 将订单信息存入活动表
		ActivityType:      activityType,
		Maker:             maker.String(),
		Taker:             ZeroAddress,
		MarketplaceID:     multi.MarketOrderBook,
		CollectionAddress: event.Nft.CollectionAddr.String(),
		TokenId:           event.Nft.TokenId.String(),
		CurrencyAddress:   s.cfg.ContractCfg.EthAddress,
		Price:             decimal.NewFromBigInt(event.Price, 0),
		BlockNumber:       int64(log.BlockNumber),
		TxHash:            log.TxHash.String(),
		EventTime:         int64(blockTime),
	}
	if err := s.db.WithContext(s.ctx).Table(multi.ActivityTableName(s.chain)).Clauses(clause.OnConflict{
		DoNothing: true,
	}).Create(&newActivity).Error; err != nil {
		xzap.WithContext(s.ctx).Warn("failed on create activity",
			zap.Error(err))
	}

	if err := s.orderManager.AddToOrderManagerQueue(&multi.Order{ // 将订单信息存入订单管理队列
		ExpireTime:        newOrder.ExpireTime,
		OrderID:           newOrder.OrderID,
		CollectionAddress: newOrder.CollectionAddress,
		TokenId:           newOrder.TokenId,
		Price:             newOrder.Price,
		Maker:             newOrder.Maker,
	}); err != nil {
		xzap.WithContext(s.ctx).Error("failed on add order to manager queue",
			zap.Error(err),
			zap.String("order_id", newOrder.OrderID))
	}
}

func (s *Service) handleMatchEvent(log ethereumTypes.Log) {
	// 添加分叉检查
	if err := s.checkAndHandleFork(log.BlockNumber, log.TxHash.String()); err != nil {
		xzap.WithContext(s.ctx).Error("failed to handle fork for match event",
			zap.Error(err),
			zap.Uint64("block_number", log.BlockNumber),
			zap.String("tx_hash", log.TxHash.String()))
		return
	}

	var event struct {
		MakeOrder Order
		TakeOrder Order
		FillPrice *big.Int
	}

	err := s.parsedAbi.UnpackIntoInterface(&event, "LogMatch", log.Data)
	if err != nil {
		xzap.WithContext(s.ctx).Error("Error unpacking LogMatch event:", zap.Error(err))
		return
	}

	makeOrderId := HexPrefix + hex.EncodeToString(log.Topics[1].Bytes()) // 通过topic获取订单ID
	takeOrderId := HexPrefix + hex.EncodeToString(log.Topics[2].Bytes())
	var owner string
	var collection string
	var tokenId string
	var from string
	var to string
	var sellOrderId string
	var buyOrder multi.Order
	if event.MakeOrder.Side == Bid { // 买单， 由卖方发起交易撮合
		owner = strings.ToLower(event.MakeOrder.Maker.String())
		collection = event.TakeOrder.Nft.CollectionAddr.String()
		tokenId = event.TakeOrder.Nft.TokenId.String()
		from = event.TakeOrder.Maker.String()
		to = event.MakeOrder.Maker.String()
		sellOrderId = takeOrderId

		// 更新卖方订单状态
		if err := s.db.WithContext(s.ctx).Table(multi.OrderTableName(s.chain)).
			Where("order_id = ?", takeOrderId).
			Updates(map[string]interface{}{
				"order_status":       multi.OrderStatusFilled,
				"quantity_remaining": 0,
				"taker":              to,
			}).Error; err != nil {
			xzap.WithContext(s.ctx).Error("failed on update order status",
				zap.String("order_id", takeOrderId))
			return
		}

		// 查询买方订单信息，不存在则无需更新，说明不是从平台前端发起的交易
		if err := s.db.WithContext(s.ctx).Table(multi.OrderTableName(s.chain)).
			Where("order_id = ?", makeOrderId).
			First(&buyOrder).Error; err != nil {
			xzap.WithContext(s.ctx).Error("failed on get buy order",
				zap.Error(err))
			return
		}
		// 更新买方订单的剩余数量
		if buyOrder.QuantityRemaining > 1 {
			if err := s.db.WithContext(s.ctx).Table(multi.OrderTableName(s.chain)).
				Where("order_id = ?", makeOrderId).
				Update("quantity_remaining", buyOrder.QuantityRemaining-1).Error; err != nil {
				xzap.WithContext(s.ctx).Error("failed on update order quantity_remaining",
					zap.String("order_id", makeOrderId))
				return
			}
		} else {
			if err := s.db.WithContext(s.ctx).Table(multi.OrderTableName(s.chain)).
				Where("order_id = ?", makeOrderId).
				Updates(map[string]interface{}{
					"order_status":       multi.OrderStatusFilled,
					"quantity_remaining": 0,
				}).Error; err != nil {
				xzap.WithContext(s.ctx).Error("failed on update order status",
					zap.String("order_id", makeOrderId))
				return
			}
		}
	} else { // 卖单， 由买方发起交易撮合， 同理
		owner = strings.ToLower(event.TakeOrder.Maker.String())
		collection = event.MakeOrder.Nft.CollectionAddr.String()
		tokenId = event.MakeOrder.Nft.TokenId.String()
		from = event.MakeOrder.Maker.String()
		to = event.TakeOrder.Maker.String()
		sellOrderId = makeOrderId

		if err := s.db.WithContext(s.ctx).Table(multi.OrderTableName(s.chain)).
			Where("order_id = ?", makeOrderId).
			Updates(map[string]interface{}{
				"order_status":       multi.OrderStatusFilled,
				"quantity_remaining": 0,
				"taker":              to,
			}).Error; err != nil {
			xzap.WithContext(s.ctx).Error("failed on update order status",
				zap.String("order_id", makeOrderId))
			return
		}

		if err := s.db.WithContext(s.ctx).Table(multi.OrderTableName(s.chain)).
			Where("order_id = ?", takeOrderId).
			First(&buyOrder).Error; err != nil {
			xzap.WithContext(s.ctx).Error("failed on get buy order",
				zap.Error(err))
			return
		}
		if buyOrder.QuantityRemaining > 1 {
			if err := s.db.WithContext(s.ctx).Table(multi.OrderTableName(s.chain)).
				Where("order_id = ?", takeOrderId).
				Update("quantity_remaining", buyOrder.QuantityRemaining-1).Error; err != nil {
				xzap.WithContext(s.ctx).Error("failed on update order quantity_remaining",
					zap.String("order_id", takeOrderId))
				return
			}
		} else {
			if err := s.db.WithContext(s.ctx).Table(multi.OrderTableName(s.chain)).
				Where("order_id = ?", takeOrderId).
				Updates(map[string]interface{}{
					"order_status":       multi.OrderStatusFilled,
					"quantity_remaining": 0,
				}).Error; err != nil {
				xzap.WithContext(s.ctx).Error("failed on update order status",
					zap.String("order_id", takeOrderId))
				return
			}
		}
	}

	blockTime, err := s.chainClient.BlockTimeByNumber(s.ctx, big.NewInt(int64(log.BlockNumber)))
	if err != nil {
		xzap.WithContext(s.ctx).Error("failed to get block time", zap.Error(err))
		return
	}
	newActivity := multi.Activity{
		ActivityType:      multi.Sale,
		Maker:             event.MakeOrder.Maker.String(),
		Taker:             event.TakeOrder.Maker.String(),
		MarketplaceID:     multi.MarketOrderBook,
		CollectionAddress: collection,
		TokenId:           tokenId,
		CurrencyAddress:   s.cfg.ContractCfg.EthAddress,
		Price:             decimal.NewFromBigInt(event.FillPrice, 0),
		BlockNumber:       int64(log.BlockNumber),
		TxHash:            log.TxHash.String(),
		EventTime:         int64(blockTime),
	}
	if err := s.db.WithContext(s.ctx).Table(multi.ActivityTableName(s.chain)).Clauses(clause.OnConflict{
		DoNothing: true,
	}).Create(&newActivity).Error; err != nil {
		xzap.WithContext(s.ctx).Warn("failed on create activity",
			zap.Error(err))
	}

	// 更新NFT的所有者
	if err := s.db.WithContext(s.ctx).Table(multi.ItemTableName(s.chain)).
		Where("LOWER(collection_address) = LOWER(?) AND token_id = ?", collection, tokenId).
		Update("owner", owner).Error; err != nil {
		xzap.WithContext(s.ctx).Error("failed to update item owner",
			zap.Error(err))
		return
	}
	// 交易撮合后，该 NFT 视为已售出/已下架。
	s.syncItemOnMatchOrCancel(collection, tokenId, decimal.NewFromBigInt(event.FillPrice, 0))

	if err := ordermanager.AddUpdatePriceEvent(s.kv, &ordermanager.TradeEvent{ // 将交易信息存入价格更新队列
		OrderId:        sellOrderId,
		CollectionAddr: collection,
		EventType:      ordermanager.Buy,
		TokenID:        tokenId,
		From:           from,
		To:             to,
	}, s.chain); err != nil {
		xzap.WithContext(s.ctx).Error("failed on add update price event",
			zap.Error(err),
			zap.String("type", "sale"),
			zap.String("order_id", sellOrderId))
	}
}

func (s *Service) handleCancelEvent(log ethereumTypes.Log) {
	// 添加分叉检查
	if err := s.checkAndHandleFork(log.BlockNumber, log.TxHash.String()); err != nil {
		xzap.WithContext(s.ctx).Error("failed to handle fork for cancel event",
			zap.Error(err),
			zap.Uint64("block_number", log.BlockNumber),
			zap.String("tx_hash", log.TxHash.String()))
		return
	}

	orderId := HexPrefix + hex.EncodeToString(log.Topics[1].Bytes())
	//maker := common.BytesToAddress(log.Topics[2].Bytes())
	if err := s.db.WithContext(s.ctx).Table(multi.OrderTableName(s.chain)).
		Where("order_id = ?", orderId).
		Update("order_status", multi.OrderStatusCancelled).Error; err != nil {
		xzap.WithContext(s.ctx).Error("failed on update order status",
			zap.String("order_id", orderId))
		return
	}

	var cancelOrder multi.Order
	if err := s.db.WithContext(s.ctx).Table(multi.OrderTableName(s.chain)).
		Where("order_id = ?", orderId).
		First(&cancelOrder).Error; err != nil {
		xzap.WithContext(s.ctx).Error("failed on get cancel order",
			zap.Error(err))
		return
	}

	blockTime, err := s.chainClient.BlockTimeByNumber(s.ctx, big.NewInt(int64(log.BlockNumber)))
	if err != nil {
		xzap.WithContext(s.ctx).Error("failed to get block time", zap.Error(err))
		return
	}
	var activityType int
	if cancelOrder.OrderType == multi.ListingOrder {
		activityType = multi.CancelListing
	} else if cancelOrder.OrderType == multi.CollectionBidOrder {
		activityType = multi.CancelCollectionBid
	} else {
		activityType = multi.CancelItemBid
	}
	newActivity := multi.Activity{
		ActivityType:      activityType,
		Maker:             cancelOrder.Maker,
		Taker:             ZeroAddress,
		MarketplaceID:     multi.MarketOrderBook,
		CollectionAddress: cancelOrder.CollectionAddress,
		TokenId:           cancelOrder.TokenId,
		CurrencyAddress:   s.cfg.ContractCfg.EthAddress,
		Price:             cancelOrder.Price,
		BlockNumber:       int64(log.BlockNumber),
		TxHash:            log.TxHash.String(),
		EventTime:         int64(blockTime),
	}
	if err := s.db.WithContext(s.ctx).Table(multi.ActivityTableName(s.chain)).Clauses(clause.OnConflict{
		DoNothing: true,
	}).Create(&newActivity).Error; err != nil {
		xzap.WithContext(s.ctx).Warn("failed on create activity",
			zap.Error(err))
	}

	if err := ordermanager.AddUpdatePriceEvent(s.kv, &ordermanager.TradeEvent{
		OrderId:        cancelOrder.OrderID,
		CollectionAddr: cancelOrder.CollectionAddress,
		TokenID:        cancelOrder.TokenId,
		EventType:      ordermanager.Cancel,
	}, s.chain); err != nil {
		xzap.WithContext(s.ctx).Error("failed on add update price event",
			zap.Error(err),
			zap.String("type", "cancel"),
			zap.String("order_id", cancelOrder.OrderID))
	}

	// 仅卖单取消才影响 item 上架状态；买单取消不改 item。
	if cancelOrder.OrderType == multi.ListingOrder {
		s.syncItemOnMatchOrCancel(cancelOrder.CollectionAddress, cancelOrder.TokenId, cancelOrder.Price)
	}
}

// 处理ERC721 Approval事件
func (s *Service) handleApprovalEvent(log ethereumTypes.Log) {
	// 添加分叉检查
	if err := s.checkAndHandleFork(log.BlockNumber, log.TxHash.String()); err != nil {
		xzap.WithContext(s.ctx).Error("failed to handle fork for approval event",
			zap.Error(err),
			zap.Uint64("block_number", log.BlockNumber),
			zap.String("tx_hash", log.TxHash.String()))
		return
	}

	// Approval 事件结构：event Approval(address indexed owner, address indexed approved, uint256 indexed tokenId)
	if len(log.Topics) < 4 {
		xzap.WithContext(s.ctx).Error("invalid approval event topics length")
		return
	}

	owner := common.BytesToAddress(log.Topics[1].Bytes())
	approved := common.BytesToAddress(log.Topics[2].Bytes())
	tokenId := new(big.Int).SetBytes(log.Topics[3].Bytes())
	collectionAddress := log.Address.String()

	// 记录授权信息
	xzap.WithContext(s.ctx).Info("ERC721 Approval event detected",
		zap.String("collection", collectionAddress),
		zap.String("token_id", tokenId.String()),
		zap.String("owner", owner.String()),
		zap.String("approved", approved.String()),
		zap.Bool("is_vault_approved", strings.EqualFold(approved.String(), s.vaultAddress)))

	// 可以在这里添加数据库存储逻辑，记录授权状态
	blockTime, err := s.chainClient.BlockTimeByNumber(s.ctx, big.NewInt(int64(log.BlockNumber)))
	if err != nil {
		xzap.WithContext(s.ctx).Error("failed to get block time", zap.Error(err))
		return
	}

	// 存储授权记录到数据库（可选）
	approvalRecord := multi.Activity{
		ActivityType:      multi.Listing, // 暂时使用现有的类型，后续可以添加新的常量
		Maker:             owner.String(),
		Taker:             approved.String(),
		MarketplaceID:     multi.MarketOrderBook,
		CollectionAddress: collectionAddress,
		TokenId:           tokenId.String(),
		CurrencyAddress:   s.cfg.ContractCfg.EthAddress,
		Price:             decimal.Zero,
		BlockNumber:       int64(log.BlockNumber),
		TxHash:            log.TxHash.String(),
		EventTime:         int64(blockTime),
	}

	if err := s.db.WithContext(s.ctx).Table(multi.ActivityTableName(s.chain)).Clauses(clause.OnConflict{
		DoNothing: true,
	}).Create(&approvalRecord).Error; err != nil {
		xzap.WithContext(s.ctx).Warn("failed on create approval activity",
			zap.Error(err))
	}
}

// CheckNFTApprovalStatus 检查指定NFT是否已授权给vault合约
func (s *Service) CheckNFTApprovalStatus(collectionAddress string, tokenId string) (bool, error) {
	// 调用 getApproved 方法
	tokenIdBig := new(big.Int)
	tokenIdBig.SetString(tokenId, 10)

	// 获取当前授权的地址
	approvedAddress, err := s.getApprovedAddress(collectionAddress, tokenIdBig)
	if err != nil {
		return false, errors.Wrap(err, "failed to get approved address")
	}

	// 检查是否授权给vault合约
	isApproved := strings.EqualFold(approvedAddress.String(), s.vaultAddress)

	xzap.WithContext(s.ctx).Info("NFT approval status checked",
		zap.String("collection", collectionAddress),
		zap.String("token_id", tokenId),
		zap.String("approved_to", approvedAddress.String()),
		zap.String("vault_address", s.vaultAddress),
		zap.Bool("is_vault_approved", isApproved))

	return isApproved, nil
}

// getApprovedAddress 获取NFT当前授权的地址
func (s *Service) getApprovedAddress(collectionAddress string, tokenId *big.Int) (common.Address, error) {
	// ERC721 getApproved 方法的方法签名
	methodSig := "0x081812fc" // getApproved(uint256)的方法签名

	// 编码参数
	paddedTokenId := common.LeftPadBytes(tokenId.Bytes(), 32)
	data := append(common.Hex2Bytes(methodSig[2:]), paddedTokenId...)

	// 调用合约
	contractAddr := common.HexToAddress(collectionAddress)
	callMsg := ethereum.CallMsg{
		To:   &contractAddr,
		Data: data,
	}

	result, err := s.chainClient.CallContract(s.ctx, callMsg, nil)
	if err != nil {
		return common.Address{}, errors.Wrap(err, "failed to call getApproved")
	}

	if len(result) < 32 {
		return common.Address{}, errors.New("invalid response length")
	}

	// 解析结果
	approvedAddress := common.BytesToAddress(result[12:32]) // 取最后20字节作为地址
	return approvedAddress, nil
}

// CheckMultipleNFTApprovals 批量检查多个NFT的授权状态
func (s *Service) CheckMultipleNFTApprovals(nfts []struct {
	CollectionAddress string
	TokenId           string
}) (map[string]bool, error) {
	results := make(map[string]bool)

	for _, nft := range nfts {
		key := fmt.Sprintf("%s:%s", nft.CollectionAddress, nft.TokenId)
		approved, err := s.CheckNFTApprovalStatus(nft.CollectionAddress, nft.TokenId)
		if err != nil {
			xzap.WithContext(s.ctx).Error("failed to check NFT approval",
				zap.String("collection", nft.CollectionAddress),
				zap.String("token_id", nft.TokenId),
				zap.Error(err))
			results[key] = false
			continue
		}
		results[key] = approved
	}

	return results, nil
}

// CanMarketBuyNFT 判断市场合约是否能购买指定的NFT
func (s *Service) CanMarketBuyNFT(collectionAddress string, tokenId string) (bool, string, error) {
	// 1. 检查NFT是否授权给vault
	isApproved, err := s.CheckNFTApprovalStatus(collectionAddress, tokenId)
	if err != nil {
		return false, "检查授权状态失败", err
	}

	if !isApproved {
		return false, "NFT未授权给vault合约", nil
	}

	// 2. 检查是否有有效的挂单
	var activeOrder multi.Order
	if err := s.db.WithContext(s.ctx).Table(multi.OrderTableName(s.chain)).
		Where("collection_address = ? AND token_id = ? AND order_type = ? AND order_status = ? AND expire_time > ?",
			strings.ToLower(collectionAddress), tokenId, multi.ListingOrder, multi.OrderStatusActive, time.Now().Unix()).
		First(&activeOrder).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, "没有有效的挂单", nil
		}
		return false, "查询挂单失败", err
	}

	// 3. 验证挂单的maker确实是NFT的owner（可选的额外安全检查）
	// 这里可以添加更多的验证逻辑

	return true, "可以购买", nil
}

// 新增处理分叉的函数
func (s *Service) checkAndHandleFork(blockNumber uint64, txHash string) error {
	// 检查交易是否已经存在
	var count int64
	if err := s.db.WithContext(s.ctx).Table(multi.ActivityTableName(s.chain)).
		Where("tx_hash = ? AND block_number != ?", txHash, blockNumber).
		Count(&count).Error; err != nil {
		return errors.Wrap(err, "failed to check transaction existence")
	}

	// 如果交易存在但区块高度不同，说明发生了分叉
	if count > 0 {
		// 1. 回滚该交易相关的订单状态
		if err := s.rollbackOrderStatus(txHash); err != nil {
			return errors.Wrap(err, "failed to rollback order status")
		}

		// 2. 删除原有的活动记录
		if err := s.db.WithContext(s.ctx).Table(multi.ActivityTableName(s.chain)).
			Where("tx_hash = ?", txHash).
			Delete(&multi.Activity{}).Error; err != nil {
			return errors.Wrap(err, "failed to delete forked activity")
		}

		xzap.WithContext(s.ctx).Info("handled fork situation",
			zap.String("tx_hash", txHash),
			zap.Uint64("new_block_number", blockNumber))
	}

	return nil
}

// 新增回滚订单状态的函数
func (s *Service) rollbackOrderStatus(txHash string) error {
	// 查找与该交易相关的活动
	var activities []multi.Activity
	if err := s.db.WithContext(s.ctx).Table(multi.ActivityTableName(s.chain)).
		Where("tx_hash = ?", txHash).
		Find(&activities).Error; err != nil {
		return errors.Wrap(err, "failed to find activities")
	}

	for _, activity := range activities {
		switch activity.ActivityType {
		case multi.Sale:
			// 对于销售活动，恢复订单状态为活跃
			if err := s.db.WithContext(s.ctx).Table(multi.OrderTableName(s.chain)).
				Where("maker = ? AND collection_address = ? AND token_id = ? AND price = ?",
					activity.Maker, activity.CollectionAddress, activity.TokenId, activity.Price).
				Updates(map[string]interface{}{
					"order_status":       multi.OrderStatusActive,
					"quantity_remaining": gorm.Expr("quantity_remaining + 1"),
					"taker":              ZeroAddress,
				}).Error; err != nil {
				return errors.Wrap(err, "failed to restore order status for sale")
			}

			// 恢复NFT所有权
			if err := s.db.WithContext(s.ctx).Table(multi.ItemTableName(s.chain)).
				Where("collection_address = ? AND token_id = ?",
					activity.CollectionAddress, activity.TokenId).
				Update("owner", activity.Maker).Error; err != nil {
				return errors.Wrap(err, "failed to restore item ownership")
			}

		case multi.CancelListing, multi.CancelCollectionBid, multi.CancelItemBid:
			// 对于取消活动，恢复订单状态为活跃
			if err := s.db.WithContext(s.ctx).Table(multi.OrderTableName(s.chain)).
				Where("maker = ? AND collection_address = ? AND token_id = ? AND price = ?",
					activity.Maker, activity.CollectionAddress, activity.TokenId, activity.Price).
				Update("order_status", multi.OrderStatusActive).Error; err != nil {
				return errors.Wrap(err, "failed to restore order status for cancel")
			}
		}
	}

	return nil
}

func (s *Service) UpKeepingCollectionFloorChangeLoop() {
	timer := time.NewTicker(comm.DaySeconds * time.Second)
	defer timer.Stop()
	updateFloorPriceTimer := time.NewTicker(comm.MaxCollectionFloorTimeDifference * time.Second)
	defer updateFloorPriceTimer.Stop()

	var indexedStatus base.IndexedStatus
	res := s.db.WithContext(s.ctx).Table(base.IndexedStatusTableName()).
		Select("last_indexed_time").
		Where("chain_id = ? and index_type = ?", s.chainId, comm.CollectionFloorChangeIndexType).
		Limit(1).
		Find(&indexedStatus)
	if res.Error != nil {
		xzap.WithContext(s.ctx).Error("failed on get collection floor change index status",
			zap.Error(res.Error))
		return
	}
	if res.RowsAffected == 0 {
		now := time.Now().Unix()
		initStatus := map[string]interface{}{
			"chain_id":           s.chainId,
			"index_type":         comm.CollectionFloorChangeIndexType,
			"last_indexed_block": 0,
			"last_indexed_time":  now,
			"create_time":        now,
			"update_time":        now,
		}
		if createErr := s.db.WithContext(s.ctx).Table(base.IndexedStatusTableName()).
			Create(initStatus).Error; createErr != nil {
			xzap.WithContext(s.ctx).Error("failed on init collection floor change index status",
				zap.Error(createErr))
			return
		}
		xzap.WithContext(s.ctx).Info("initialized collection floor change index status",
			zap.Int64("chain_id", s.chainId),
			zap.Int("index_type", comm.CollectionFloorChangeIndexType))
	}

	for {
		select {
		case <-s.ctx.Done():
			xzap.WithContext(s.ctx).Info("UpKeepingCollectionFloorChangeLoop stopped due to context cancellation")
			return
		case <-timer.C:
			if err := s.deleteExpireCollectionFloorChangeFromDatabase(); err != nil {
				xzap.WithContext(s.ctx).Error("failed on delete expire collection floor change",
					zap.Error(err))
			}
		case <-updateFloorPriceTimer.C:
			if s.cfg.ProjectCfg.Name == gdb.OrderBookDexProject {
				floorPrices, err := s.QueryCollectionsFloorPrice()
				if err != nil {
					xzap.WithContext(s.ctx).Error("failed on query collections floor change",
						zap.Error(err))
					continue
				}

				if err := s.persistCollectionsFloorChange(floorPrices); err != nil {
					xzap.WithContext(s.ctx).Error("failed on persist collections floor price",
						zap.Error(err))
					continue
				}
			}
		}
	}
}

func (s *Service) deleteExpireCollectionFloorChangeFromDatabase() error {
	stmt := fmt.Sprintf(`DELETE FROM %s where event_time < UNIX_TIMESTAMP() - %d`, gdb.GetMultiProjectCollectionFloorPriceTableName(s.cfg.ProjectCfg.Name, s.chain), comm.CollectionFloorTimeRange)

	if err := s.db.Exec(stmt).Error; err != nil {
		return errors.Wrap(err, "failed on delete expire collection floor price")
	}

	return nil
}

func (s *Service) QueryCollectionsFloorPrice() ([]multi.CollectionFloorPrice, error) {
	timestamp := time.Now().Unix()
	timestampMilli := time.Now().UnixMilli()
	var collectionFloorPrice []multi.CollectionFloorPrice
	sql := fmt.Sprintf(`SELECT co.collection_address as collection_address,min(co.price) as price
FROM %s as ci
         left join %s co on co.collection_address = ci.collection_address and co.token_id = ci.token_id
WHERE (co.order_type = ? and
       co.order_status = ? and expire_time > ? and lower(co.maker) = lower(ci.owner)) group by co.collection_address`, gdb.GetMultiProjectItemTableName(s.cfg.ProjectCfg.Name, s.chain), gdb.GetMultiProjectOrderTableName(s.cfg.ProjectCfg.Name, s.chain))
	if err := s.db.WithContext(s.ctx).Raw(
		sql,
		multi.ListingType,
		multi.OrderStatusActive,
		time.Now().Unix(),
	).Scan(&collectionFloorPrice).Error; err != nil {
		return nil, errors.Wrap(err, "failed on get collection floor price")
	}

	for i := 0; i < len(collectionFloorPrice); i++ {
		collectionFloorPrice[i].EventTime = timestamp
		collectionFloorPrice[i].CreateTime = timestampMilli
		collectionFloorPrice[i].UpdateTime = timestampMilli
	}

	return collectionFloorPrice, nil
}

func (s *Service) persistCollectionsFloorChange(FloorPrices []multi.CollectionFloorPrice) error {
	for i := 0; i < len(FloorPrices); i += comm.DBBatchSizeLimit {
		end := i + comm.DBBatchSizeLimit
		if i+comm.DBBatchSizeLimit >= len(FloorPrices) {
			end = len(FloorPrices)
		}

		valueStrings := make([]string, 0)
		valueArgs := make([]interface{}, 0)

		for _, t := range FloorPrices[i:end] {
			valueStrings = append(valueStrings, "(?,?,?,?,?)")
			valueArgs = append(valueArgs, t.CollectionAddress, t.Price, t.EventTime, t.CreateTime, t.UpdateTime)
		}

		stmt := fmt.Sprintf(`INSERT INTO %s (collection_address,price,event_time,create_time,update_time)  VALUES %s
		ON DUPLICATE KEY UPDATE update_time=VALUES(update_time)`, gdb.GetMultiProjectCollectionFloorPriceTableName(s.cfg.ProjectCfg.Name, s.chain), strings.Join(valueStrings, ","))

		if err := s.db.Exec(stmt, valueArgs...).Error; err != nil {
			return errors.Wrap(err, "failed on persist collection floor price info")
		}
	}
	return nil
}

// maintainCollectionAndItem 维护 collection 和 item 信息
func (s *Service) maintainCollectionAndItem(collectionAddress, tokenId string, price decimal.Decimal) {
	// 1. 检查并创建 collection 记录（如果不存在）
	s.ensureCollectionExists(collectionAddress, tokenId)

	// 2. 更新 item 的上架信息
	s.updateItemListingInfo(collectionAddress, tokenId, price)

	// 3. 更新 collection 的 floor_price
	s.updateCollectionFloorPrice(collectionAddress)
}

// ensureCollectionExists 确保 collection 记录存在
func (s *Service) ensureCollectionExists(collectionAddress, tokenId string) {
	collectionTableName := gdb.GetMultiProjectCollectionTableName(s.cfg.ProjectCfg.Name, s.chain)

	// 检查 collection 是否存在
	var count int64
	if err := s.db.WithContext(s.ctx).Table(collectionTableName).
		Where("address = ?", collectionAddress).
		Count(&count).Error; err != nil {
		xzap.WithContext(s.ctx).Error("failed to check collection existence",
			zap.Error(err),
			zap.String("collection_address", collectionAddress))
		return
	}

	name, symbol, tokenStandard, creator, metaErr := s.fetchCollectionMeta(collectionAddress)
	if metaErr != nil {
		xzap.WithContext(s.ctx).Warn("failed to fetch collection metadata from chain, fallback to defaults",
			zap.Error(metaErr),
			zap.String("collection_address", collectionAddress))
	}
	collectionImageURI := s.getCollectionImageFromItem(collectionAddress, tokenId)

	// 如果不存在，创建 collection 记录
	if count == 0 {
		now := time.Now().Unix()
		collection := map[string]interface{}{
			"address":        collectionAddress,
			"chain_id":       s.chainId,
			"symbol":         symbol,
			"name":           name,
			"image_uri":      collectionImageURI,
			"creator":        creator,
			"token_standard": tokenStandard,
			"auth":           0, // 业务认证字段，非链上信息
			"owner_amount":   0, // 统计字段，后续由聚合逻辑维护
			"item_amount":    0, // 统计字段，后续由聚合逻辑维护
			"floor_price":    nil,
			"sale_price":     nil,
			"volume_total":   decimal.Zero,
			"create_time":    now,
			"update_time":    now,
		}

		if err := s.db.WithContext(s.ctx).Table(collectionTableName).
			Create(&collection).Error; err != nil {
			xzap.WithContext(s.ctx).Error("failed to create collection record",
				zap.Error(err),
				zap.String("collection_address", collectionAddress))
		} else {
			xzap.WithContext(s.ctx).Info("created new collection record",
				zap.String("collection_address", collectionAddress))
		}
		return
	}

	// collection 已存在时，如果仍是 Unknown，尝试回填链上 name/symbol。
	var existing struct {
		Name          string `gorm:"column:name"`
		Symbol        string `gorm:"column:symbol"`
		ImageURI      string `gorm:"column:image_uri"`
		Creator       string `gorm:"column:creator"`
		TokenStandard int64  `gorm:"column:token_standard"`
	}
	if err := s.db.WithContext(s.ctx).Table(collectionTableName).
		Select("name, symbol, image_uri, creator, token_standard").
		Where("address = ?", collectionAddress).
		Take(&existing).Error; err != nil {
		xzap.WithContext(s.ctx).Error("failed to query existing collection metadata",
			zap.Error(err),
			zap.String("collection_address", collectionAddress))
		return
	}

	updates := map[string]interface{}{}
	if isUnknownCollectionName(existing.Name) && !isUnknownCollectionName(name) {
		updates["name"] = name
	}
	if isUnknownCollectionSymbol(existing.Symbol) && !isUnknownCollectionSymbol(symbol) {
		updates["symbol"] = symbol
	}
	if strings.TrimSpace(existing.ImageURI) == "" && strings.TrimSpace(collectionImageURI) != "" {
		updates["image_uri"] = collectionImageURI
	}
	if isZeroAddress(existing.Creator) && !isZeroAddress(creator) {
		updates["creator"] = creator
	}
	if existing.TokenStandard == 0 && tokenStandard != 0 {
		updates["token_standard"] = tokenStandard
	}
	if len(updates) == 0 {
		return
	}
	updates["update_time"] = time.Now().Unix()

	if err := s.db.WithContext(s.ctx).Table(collectionTableName).
		Where("address = ?", collectionAddress).
		Updates(updates).Error; err != nil {
		xzap.WithContext(s.ctx).Error("failed to backfill collection metadata",
			zap.Error(err),
			zap.String("collection_address", collectionAddress))
	} else {
		xzap.WithContext(s.ctx).Info("backfilled collection metadata",
			zap.String("collection_address", collectionAddress),
			zap.Any("updates", updates))
	}
}

func (s *Service) getCollectionImageFromItem(collectionAddress, tokenId string) string {
	itemExternalTableName := fmt.Sprintf("ob_item_external_%s", s.chain)

	var itemExternal struct {
		ImageURI string `gorm:"column:image_uri"`
	}
	if err := s.db.WithContext(s.ctx).Table(itemExternalTableName).
		Select("image_uri").
		Where("collection_address = ? AND token_id = ?", collectionAddress, tokenId).
		Limit(1).
		Take(&itemExternal).Error; err == nil && strings.TrimSpace(itemExternal.ImageURI) != "" {
		return itemExternal.ImageURI
	}

	// 回退到 item 表里的 image_uri
	itemTableName := gdb.GetMultiProjectItemTableName(s.cfg.ProjectCfg.Name, s.chain)
	var item struct {
		ImageURI string `gorm:"column:image_uri"`
	}
	if err := s.db.WithContext(s.ctx).Table(itemTableName).
		Select("image_uri").
		Where("collection_address = ? AND token_id = ?", collectionAddress, tokenId).
		Limit(1).
		Take(&item).Error; err == nil && strings.TrimSpace(item.ImageURI) != "" {
		return item.ImageURI
	}

	return ""
}

func isUnknownCollectionName(name string) bool {
	normalized := strings.ToLower(strings.TrimSpace(name))
	return normalized == "" || normalized == "unknown collection" || normalized == "unknown"
}

func isUnknownCollectionSymbol(symbol string) bool {
	normalized := strings.ToLower(strings.TrimSpace(symbol))
	return normalized == "" || normalized == "unknown"
}

func isZeroAddress(addr string) bool {
	return strings.EqualFold(strings.TrimSpace(addr), ZeroAddress)
}

func (s *Service) fetchCollectionMeta(collectionAddress string) (string, string, int64, string, error) {
	const (
		defaultName   = "Unknown Collection"
		defaultSymbol = "Unknown"
		defaultStd    = int64(721)
	)

	name, nameErr := s.callContractStringNoArgs(collectionAddress, "0x06fdde03")  // name()
	symbol, symErr := s.callContractStringNoArgs(collectionAddress, "0x95d89b41") // symbol()
	tokenStandard, stdErr := s.detectTokenStandard(collectionAddress)
	creator, creatorErr := s.callContractAddressNoArgs(collectionAddress, "0x8da5cb5b") // owner()

	if strings.TrimSpace(name) == "" {
		name = defaultName
	}
	if strings.TrimSpace(symbol) == "" {
		symbol = defaultSymbol
	}
	if tokenStandard == 0 {
		tokenStandard = defaultStd
	}
	if isZeroAddress(creator) {
		creator = ZeroAddress
	}

	// owner() 常见于 Ownable 合约；若不存在不视为致命错误，仅用默认值。
	if creatorErr != nil {
		creator = ZeroAddress
	}

	if nameErr != nil && symErr != nil && stdErr != nil {
		return name, symbol, tokenStandard, creator, errors.Errorf(
			"name()/symbol()/supportsInterface() all failed, nameErr: %v, symbolErr: %v, stdErr: %v",
			nameErr, symErr, stdErr,
		)
	}

	return name, symbol, tokenStandard, creator, nil
}

func (s *Service) callContractStringNoArgs(contractAddress, methodSig string) (string, error) {
	if !strings.HasPrefix(methodSig, HexPrefix) || len(methodSig) != 10 {
		return "", errors.Errorf("invalid method signature: %s", methodSig)
	}

	contractAddr := common.HexToAddress(contractAddress)
	callMsg := ethereum.CallMsg{
		To:   &contractAddr,
		Data: common.Hex2Bytes(methodSig[2:]),
	}

	result, err := s.chainClient.CallContract(s.ctx, callMsg, nil)
	if err != nil {
		return "", err
	}

	if len(result) < 64 {
		return "", errors.New("invalid response length")
	}

	offset := new(big.Int).SetBytes(result[0:32]).Uint64()
	if len(result) < int(offset+32) {
		return "", errors.New("invalid offset in response")
	}

	length := new(big.Int).SetBytes(result[offset : offset+32]).Uint64()
	if len(result) < int(offset+32+length) {
		return "", errors.New("invalid string length in response")
	}

	value := string(result[offset+32 : offset+32+length])
	value = strings.TrimSpace(strings.TrimRight(value, "\x00"))
	return value, nil
}

func (s *Service) callContractAddressNoArgs(contractAddress, methodSig string) (string, error) {
	if !strings.HasPrefix(methodSig, HexPrefix) || len(methodSig) != 10 {
		return "", errors.Errorf("invalid method signature: %s", methodSig)
	}

	contractAddr := common.HexToAddress(contractAddress)
	callMsg := ethereum.CallMsg{
		To:   &contractAddr,
		Data: common.Hex2Bytes(methodSig[2:]),
	}

	result, err := s.chainClient.CallContract(s.ctx, callMsg, nil)
	if err != nil {
		return "", err
	}
	if len(result) < 32 {
		return "", errors.New("invalid response length")
	}

	addr := common.BytesToAddress(result[len(result)-20:])
	return addr.Hex(), nil
}

func (s *Service) callSupportsInterface(contractAddress, interfaceID string) (bool, error) {
	if !strings.HasPrefix(interfaceID, HexPrefix) || len(interfaceID) != 10 {
		return false, errors.Errorf("invalid interface id: %s", interfaceID)
	}

	methodSig := "0x01ffc9a7" // supportsInterface(bytes4)
	data := append(common.Hex2Bytes(methodSig[2:]), common.Hex2Bytes(interfaceID[2:])...)
	data = append(data, make([]byte, 28)...)

	contractAddr := common.HexToAddress(contractAddress)
	callMsg := ethereum.CallMsg{
		To:   &contractAddr,
		Data: data,
	}

	result, err := s.chainClient.CallContract(s.ctx, callMsg, nil)
	if err != nil {
		return false, err
	}
	if len(result) < 32 {
		return false, errors.New("invalid response length")
	}

	return result[len(result)-1] == 1, nil
}

func (s *Service) detectTokenStandard(collectionAddress string) (int64, error) {
	const (
		erc721ID  = "0x80ac58cd"
		erc1155ID = "0xd9b67a26"
	)

	is721, err721 := s.callSupportsInterface(collectionAddress, erc721ID)
	is1155, err1155 := s.callSupportsInterface(collectionAddress, erc1155ID)

	if err721 != nil && err1155 != nil {
		return 0, errors.Errorf("supportsInterface failed, erc721Err: %v, erc1155Err: %v", err721, err1155)
	}
	if is1155 {
		return 1155, nil
	}
	if is721 {
		return 721, nil
	}

	// 无法识别时按当前业务默认 721 兜底，避免影响现有流程。
	return 721, nil
}

// updateItemListingInfo 更新 item 的上架信息
func (s *Service) updateItemListingInfo(collectionAddress, tokenId string, price decimal.Decimal) {
	itemTableName := gdb.GetMultiProjectItemTableName(s.cfg.ProjectCfg.Name, s.chain)
	now := time.Now().Unix()

	// 更新或插入 item 记录
	updates := map[string]interface{}{
		"list_price":  price,
		"list_time":   now,
		"update_time": now,
	}

	// 先尝试更新
	result := s.db.WithContext(s.ctx).Table(itemTableName).
		Where("collection_address = ? AND token_id = ?", collectionAddress, tokenId).
		Updates(updates)

	if result.Error != nil {
		xzap.WithContext(s.ctx).Error("failed to update item listing info",
			zap.Error(result.Error),
			zap.String("collection_address", collectionAddress),
			zap.String("token_id", tokenId))
		return
	}

	// 如果没有更新任何记录，说明 item 不存在，创建基础记录
	if result.RowsAffected == 0 {
		item := map[string]interface{}{
			"chain_id":           s.chainId,
			"token_id":           tokenId,
			"collection_address": collectionAddress,
			"name":               fmt.Sprintf("Token #%s", tokenId),
			"creator":            "0x0000000000000000000000000000000000000000",
			"owner":              nil, // 需要通过其他方式获取
			"supply":             1,
			"list_price":         price,
			"list_time":          now,
			"sale_price":         nil,
			"views":              0,
			"is_opensea_banned":  false,
			"create_time":        now,
			"update_time":        now,
		}

		if err := s.db.WithContext(s.ctx).Table(itemTableName).
			Create(&item).Error; err != nil {
			xzap.WithContext(s.ctx).Error("failed to create item record",
				zap.Error(err),
				zap.String("collection_address", collectionAddress),
				zap.String("token_id", tokenId))
		} else {
			xzap.WithContext(s.ctx).Info("created new item record",
				zap.String("collection_address", collectionAddress),
				zap.String("token_id", tokenId))
		}
	}
}

// updateCollectionFloorPrice 更新 collection 的 floor_price
func (s *Service) updateCollectionFloorPrice(collectionAddress string) {
	itemTableName := gdb.GetMultiProjectItemTableName(s.cfg.ProjectCfg.Name, s.chain)
	collectionTableName := gdb.GetMultiProjectCollectionTableName(s.cfg.ProjectCfg.Name, s.chain)

	// 查询该 collection 中最低的上架价格
	var minPrice decimal.Decimal
	if err := s.db.WithContext(s.ctx).Table(itemTableName).
		Select("MIN(list_price)").
		Where("collection_address = ? AND list_price IS NOT NULL AND list_price > 0", collectionAddress).
		Scan(&minPrice).Error; err != nil {
		xzap.WithContext(s.ctx).Error("failed to get collection min price",
			zap.Error(err),
			zap.String("collection_address", collectionAddress))
		return
	}

	// 更新 collection 的 floor_price
	if err := s.db.WithContext(s.ctx).Table(collectionTableName).
		Where("address = ?", collectionAddress).
		Update("floor_price", minPrice).Error; err != nil {
		xzap.WithContext(s.ctx).Error("failed to update collection floor price",
			zap.Error(err),
			zap.String("collection_address", collectionAddress),
			zap.String("floor_price", minPrice.String()))
	} else {
		xzap.WithContext(s.ctx).Debug("updated collection floor price",
			zap.String("collection_address", collectionAddress),
			zap.String("floor_price", minPrice.String()))
	}
}

// syncItemOnListing 在 LogMake(卖单) 发生时，同步 item 的价格与在售状态。
func (s *Service) syncItemOnListing(collectionAddress, tokenId, maker string, price decimal.Decimal) {
	itemTableName := gdb.GetMultiProjectItemTableName(s.cfg.ProjectCfg.Name, s.chain)
	now := time.Now().Unix()
	updates := map[string]interface{}{
		"owner":      maker,
		"list_price": price,
		// 历史上部分查询依赖 sale_price，这里和挂单价保持一致，避免前后端取值不一致。
		"sale_price": price,
		"supply":     1,
		"list_time":  now,
		"update_time": now,
	}

	if err := s.db.WithContext(s.ctx).Table(itemTableName).
		Where("LOWER(collection_address) = LOWER(?) AND token_id = ?", collectionAddress, tokenId).
		Updates(updates).Error; err != nil {
		xzap.WithContext(s.ctx).Error("failed to sync item on listing",
			zap.Error(err),
			zap.String("collection_address", collectionAddress),
			zap.String("token_id", tokenId))
	}
}

// syncItemOnMatchOrCancel 在 LogMatch/LogCancel(卖单) 后同步 item 的售出/下架状态。
func (s *Service) syncItemOnMatchOrCancel(collectionAddress, tokenId string, salePrice decimal.Decimal) {
	itemTableName := gdb.GetMultiProjectItemTableName(s.cfg.ProjectCfg.Name, s.chain)
	updates := map[string]interface{}{
		"supply":      0,
		"list_price":  decimal.Zero,
		"sale_price":  salePrice,
		"update_time": time.Now().Unix(),
	}

	if err := s.db.WithContext(s.ctx).Table(itemTableName).
		Where("LOWER(collection_address) = LOWER(?) AND token_id = ?", collectionAddress, tokenId).
		Updates(updates).Error; err != nil {
		xzap.WithContext(s.ctx).Error("failed to sync item on match/cancel",
			zap.Error(err),
			zap.String("collection_address", collectionAddress),
			zap.String("token_id", tokenId))
	}
}

// getTokenURI 获取NFT的tokenURI（元数据URI）
func (s *Service) getTokenURI(collectionAddress string, tokenId *big.Int) (string, error) {
	// ERC721 tokenURI 方法的方法签名: tokenURI(uint256)
	methodSig := "0xc87b56dd" // tokenURI(uint256)的方法签名

	// 编码参数
	paddedTokenId := common.LeftPadBytes(tokenId.Bytes(), 32)
	data := append(common.Hex2Bytes(methodSig[2:]), paddedTokenId...)

	// 调用合约
	contractAddr := common.HexToAddress(collectionAddress)
	callMsg := ethereum.CallMsg{
		To:   &contractAddr,
		Data: data,
	}

	result, err := s.chainClient.CallContract(s.ctx, callMsg, nil)
	if err != nil {
		return "", errors.Wrap(err, "failed to call tokenURI")
	}

	if len(result) < 32 {
		return "", errors.New("invalid response length")
	}

	// 解析结果：tokenURI返回的是string类型（动态类型）
	// 前32字节是offset（指向实际数据的位置）
	offset := new(big.Int).SetBytes(result[0:32]).Uint64()

	// 如果offset为0或超出结果长度，说明格式不对
	if offset == 0 || len(result) < int(offset+32) {
		return "", errors.New("invalid offset in tokenURI response")
	}

	// offset位置的前32字节是字符串长度
	length := new(big.Int).SetBytes(result[offset : offset+32]).Uint64()

	// 检查长度是否合理（不超过剩余数据）
	if length == 0 || len(result) < int(offset+32+length) {
		return "", errors.New("invalid string length in tokenURI response")
	}

	// 提取字符串数据（从offset+32开始，长度为length）
	strBytes := result[offset+32 : offset+32+length]
	tokenURI := string(strBytes)

	// 移除可能的空字符和空白字符
	tokenURI = strings.TrimRight(tokenURI, "\x00")
	tokenURI = strings.TrimSpace(tokenURI)

	return tokenURI, nil
}

// getImageFromMetadata 从元数据URI获取JSON并提取image字段
func (s *Service) getImageFromMetadata(metaDataURI string) (string, error) {
	// 创建HTTP请求，设置超时
	ctx, cancel := context.WithTimeout(s.ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", metaDataURI, nil)
	if err != nil {
		return "", errors.Wrap(err, "failed to create HTTP request")
	}

	// 设置请求头
	req.Header.Set("User-Agent", "EasySwapSync/1.0")
	req.Header.Set("Accept", "application/json")

	// 发送请求
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", errors.Wrap(err, "failed to fetch metadata")
	}
	defer resp.Body.Close()

	// 检查状态码
	if resp.StatusCode != http.StatusOK {
		return "", errors.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "failed to read response body")
	}

	// 解析JSON
	var metadata map[string]interface{}
	if err := json.Unmarshal(body, &metadata); err != nil {
		return "", errors.Wrap(err, "failed to parse JSON")
	}

	// 提取image字段
	image, ok := metadata["image"]
	if !ok {
		return "", errors.New("image field not found in metadata")
	}

	// 转换为字符串
	imageStr, ok := image.(string)
	if !ok {
		return "", errors.New("image field is not a string")
	}

	return imageStr, nil
}

// createItemExternal 创建 item_external 记录
// 表结构: ob_item_external_{chain}
// 唯一索引: (collection_address, token_id)
func (s *Service) createItemExternal(collectionAddress, tokenId string) {
	itemExternalTableName := fmt.Sprintf("ob_item_external_%s", s.chain)
	now := time.Now().Unix()

	// 尝试获取tokenURI（元数据URI）和image_uri
	var metaDataURI string
	var imageURI string
	tokenIdBig := new(big.Int)
	if _, ok := tokenIdBig.SetString(tokenId, 10); ok {
		uri, err := s.getTokenURI(collectionAddress, tokenIdBig)
		if err != nil {
			xzap.WithContext(s.ctx).Warn("failed to get tokenURI",
				zap.Error(err),
				zap.String("collection_address", collectionAddress),
				zap.String("token_id", tokenId))
			// 即使获取失败也继续创建记录，metaDataURI为空
		} else {
			metaDataURI = uri
			// 限制长度，避免超过varchar(512)
			if len(metaDataURI) > 512 {
				metaDataURI = metaDataURI[:512]
			}

			// 从tokenURI获取元数据并提取image字段
			if metaDataURI != "" {
				image, err := s.getImageFromMetadata(metaDataURI)
				if err != nil {
					xzap.WithContext(s.ctx).Warn("failed to get image from metadata",
						zap.Error(err),
						zap.String("meta_data_uri", metaDataURI),
						zap.String("collection_address", collectionAddress),
						zap.String("token_id", tokenId))
					// 即使获取失败也继续，imageURI为空
				} else {
					imageURI = image
					// 限制长度，避免超过varchar(512)
					if len(imageURI) > 512 {
						imageURI = imageURI[:512]
					}
				}
			}
		}
	}

	// 创建 item_external 记录
	// 根据DDL: id是自增主键，不需要设置
	// 唯一索引 (collection_address, token_id) 由 OnConflict 处理
	itemExternal := map[string]interface{}{
		"collection_address":  collectionAddress,
		"token_id":            tokenId,
		"meta_data_uri":       metaDataURI, // varchar(512)，可能为NULL
		"image_uri":           imageURI,    // varchar(512)，从元数据JSON中提取的image字段
		"is_uploaded_oss":     false,       // tinyint(1) DEFAULT '0'
		"upload_status":       0,           // tinyint NOT NULL DEFAULT '0'
		"is_video_uploaded":   false,       // tinyint(1) DEFAULT '0'
		"video_upload_status": 0,           // tinyint NOT NULL DEFAULT '0'
		"video_type":          "0",         // varchar(64) NOT NULL DEFAULT '0'
		"create_time":         now,         // bigint
		"update_time":         now,         // bigint
		// 以下字段为NULL，不设置：
		// oss_uri, video_uri, video_oss_uri
	}

	// 使用 OnConflict 处理唯一索引冲突。
	// NOTE: 在部分 MySQL + GORM 组合下，DoNothing 可能生成不完整 SQL（导致 1064）。
	// 这里改为 no-op update，确保 SQL 在 MySQL 下始终合法。
	if err := s.db.WithContext(s.ctx).Table(itemExternalTableName).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "collection_address"}, {Name: "token_id"}},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"update_time": gorm.Expr("update_time"),
			}),
		}).Create(&itemExternal).Error; err != nil {
		xzap.WithContext(s.ctx).Error("failed to create item_external record",
			zap.Error(err),
			zap.String("collection_address", collectionAddress),
			zap.String("token_id", tokenId))
	} else {
		xzap.WithContext(s.ctx).Debug("created item_external record",
			zap.String("collection_address", collectionAddress),
			zap.String("token_id", tokenId),
			zap.String("meta_data_uri", metaDataURI),
			zap.String("image_uri", imageURI))
	}
}
