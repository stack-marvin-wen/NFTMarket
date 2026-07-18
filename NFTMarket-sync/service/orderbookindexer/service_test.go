package orderbookindexer

import (
	"context"
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/ProjectsTask/EasySwapBase/chain/chainclient"
	"github.com/ProjectsTask/EasySwapBase/chain/types"
	"github.com/ProjectsTask/EasySwapBase/stores/gdb"
	"github.com/ethereum/go-ethereum/common"
	ethereumTypes "github.com/ethereum/go-ethereum/core/types"

	"github.com/ProjectsTask/EasySwapSync/model"
	"github.com/ProjectsTask/EasySwapSync/service/config"
)

func TestSyncEvent(t *testing.T) {
	ctx := context.Background()
	db := model.NewDB(&gdb.Config{
		User:         "orderbook",
		Password:     "SM1tnJjhVnDWUbqO",
		Host:         "a95467a044b9f4f1399fa1221969c7fd-799134117.us-east-1.elb.amazonaws.com",
		Port:         4000,
		Database:     "orderbook_test",
		MaxIdleConns: 10,
		MaxOpenConns: 1500,
	})

	chainClient, _ := chainclient.New(10, "https://rpc.ankr.com/optimism/9c6c678ebcb56da1cb80f7632c7c02264831232c3d53453c7726a611e7ca36d7")
	orderbookSyncer := New(ctx, nil, db, nil, chainClient, 10, "optimism", nil)

	query := types.FilterQuery{
		FromBlock: new(big.Int).SetUint64(111819366),
		ToBlock:   new(big.Int).SetUint64(111819366),
		Addresses: []string{"0x7d29d1860bD4d3A74bBD9a03C9B043d375311dCb"},
	}

	logs, _ := chainClient.FilterLogs(ctx, query)

	for _, log := range logs {
		ethLog := log.(ethereumTypes.Log)
		switch ethLog.Topics[0].String() {
		case LogMakeTopic:
			orderbookSyncer.handleMakeEvent(ethLog)
		case LogCancelTopic:
			orderbookSyncer.handleCancelEvent(ethLog)
		case LogMatchTopic:
			orderbookSyncer.handleMatchEvent(ethLog)
		default:

		}
	}
}

func TestHandleMakeEvent(t *testing.T) {
	ctx := context.Background()
	db := model.NewDB(&gdb.Config{
		User:         "orderbook",
		Password:     "SM1tnJjhVnDWUbqO",
		Host:         "a95467a044b9f4f1399fa1221969c7fd-799134117.us-east-1.elb.amazonaws.com",
		Port:         4000,
		Database:     "orderbook_test",
		MaxIdleConns: 10,
		MaxOpenConns: 1500,
	})
	chainClient, _ := chainclient.New(10, "https://rpc.ankr.com/optimism/9c6c678ebcb56da1cb80f7632c7c02264831232c3d53453c7726a611e7ca36d7")
	orderbookSyncer := New(ctx, nil, db, nil, chainClient, 10, "optimism", nil)
	data, _ := hex.DecodeString("c773ae81bc9a186dc6c5d70a486730a6f734578ae1a0116acd0aaaf69250d2650000000000000000000000000000000000000000000000000000000000000000000000000000000000000000e7f1725e7734ce288f8367e1bb143e90bb3f05120000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000002386f26fc10000000000000000000000000000000000000000000000000000000000006558875d0000000000000000000000000000000000000000000000000000000000000001")
	log := ethereumTypes.Log{
		Address: common.HexToAddress("0x123"),
		Topics: []common.Hash{
			common.HexToHash("0xfc37f2ff950f95913eb7182357ba3c14df60ef354bc7d6ab1ba2815f249fffe6"),
			common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000"),
			common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000001"),
			common.HexToHash("0x000000000000000000000000f39fd6e51aad88f6f4ce6ab8827279cfffb92266"),
		},
		Data:        data,
		BlockNumber: 111482956,
		TxHash:      common.HexToHash("0x000000000000000000000000f39fd6e51aad88f6f4ce6ab8827279cfffb92266"),
	}
	orderbookSyncer.handleMakeEvent(log)
}

func TestHandleApprovalEvent(t *testing.T) {
	ctx := context.Background()
	db := model.NewDB(&gdb.Config{
		User:         "orderbook",
		Password:     "SM1tnJjhVnDWUbqO",
		Host:         "a95467a044b9f4f1399fa1221969c7fd-799134117.us-east-1.elb.amazonaws.com",
		Port:         4000,
		Database:     "orderbook_test",
		MaxIdleConns: 10,
		MaxOpenConns: 1500,
	})
	chainClient, _ := chainclient.New(10, "https://rpc.ankr.com/optimism/9c6c678ebcb56da1cb80f7632c7c02264831232c3d53453c7726a611e7ca36d7")
	orderbookSyncer := New(ctx, nil, db, nil, chainClient, 10, "optimism", nil)

	// 模拟 ERC721 Approval 事件
	// event Approval(address indexed owner, address indexed approved, uint256 indexed tokenId)
	log := ethereumTypes.Log{
		Address: common.HexToAddress("0xe7f1725e7734ce288f8367e1bb143e90bb3f0512"), // NFT合约地址
		Topics: []common.Hash{
			common.HexToHash("0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925"), // Approval事件的topic
			common.HexToHash("0x000000000000000000000000f39fd6e51aad88f6f4ce6ab8827279cfffb92266"), // owner地址
			common.HexToHash("0x0000000000000000000000007d29d1860bd4d3a74bbd9a03c9b043d375311dcb"), // approved地址(vault地址)
			common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000001"), // tokenId
		},
		Data:        []byte{}, // Approval事件没有额外数据
		BlockNumber: 111482957,
		TxHash:      common.HexToHash("0x111000000000000000000000f39fd6e51aad88f6f4ce6ab8827279cfffb92266"),
	}

	orderbookSyncer.handleApprovalEvent(log)
}

func TestCheckNFTApprovalStatus(t *testing.T) {
	ctx := context.Background()
	db := model.NewDB(&gdb.Config{
		User:         "orderbook",
		Password:     "SM1tnJjhVnDWUbqO",
		Host:         "a95467a044b9f4f1399fa1221969c7fd-799134117.us-east-1.elb.amazonaws.com",
		Port:         4000,
		Database:     "orderbook_test",
		MaxIdleConns: 10,
		MaxOpenConns: 1500,
	})
	chainClient, _ := chainclient.New(10, "https://rpc.ankr.com/optimism/9c6c678ebcb56da1cb80f7632c7c02264831232c3d53453c7726a611e7ca36d7")

	// 设置测试配置
	testConfig := &config.Config{
		ContractCfg: config.ContractCfg{
			VaultAddress: "0x7d29d1860bD4d3A74bBD9a03C9B043d375311dCb", // 测试vault地址
		},
	}

	orderbookSyncer := New(ctx, testConfig, db, nil, chainClient, 10, "optimism", nil)

	// 测试检查NFT授权状态
	collectionAddress := "0xe7f1725e7734ce288f8367e1bb143e90bb3f0512"
	tokenId := "1"

	isApproved, err := orderbookSyncer.CheckNFTApprovalStatus(collectionAddress, tokenId)
	if err != nil {
		t.Logf("检查授权状态出错: %v", err)
	} else {
		t.Logf("NFT %s:%s 是否已授权给vault: %v", collectionAddress, tokenId, isApproved)
	}
}

func TestCanMarketBuyNFT(t *testing.T) {
	ctx := context.Background()
	db := model.NewDB(&gdb.Config{
		User:         "orderbook",
		Password:     "SM1tnJjhVnDWUbqO",
		Host:         "a95467a044b9f4f1399fa1221969c7fd-799134117.us-east-1.elb.amazonaws.com",
		Port:         4000,
		Database:     "orderbook_test",
		MaxIdleConns: 10,
		MaxOpenConns: 1500,
	})
	chainClient, _ := chainclient.New(10, "https://rpc.ankr.com/optimism/9c6c678ebcb56da1cb80f7632c7c02264831232c3d53453c7726a611e7ca36d7")

	// 设置测试配置
	testConfig := &config.Config{
		ContractCfg: config.ContractCfg{
			VaultAddress: "0x7d29d1860bD4d3A74bBD9a03C9B043d375311dCb", // 测试vault地址
		},
	}

	orderbookSyncer := New(ctx, testConfig, db, nil, chainClient, 10, "optimism", nil)

	// 测试判断是否可以购买NFT
	collectionAddress := "0xe7f1725e7734ce288f8367e1bb143e90bb3f0512"
	tokenId := "1"

	canBuy, reason, err := orderbookSyncer.CanMarketBuyNFT(collectionAddress, tokenId)
	if err != nil {
		t.Logf("检查是否可购买出错: %v", err)
	} else {
		t.Logf("NFT %s:%s 是否可购买: %v, 原因: %s", collectionAddress, tokenId, canBuy, reason)
	}
}

func TestCheckMultipleNFTApprovals(t *testing.T) {
	ctx := context.Background()
	db := model.NewDB(&gdb.Config{
		User:         "orderbook",
		Password:     "SM1tnJjhVnDWUbqO",
		Host:         "a95467a044b9f4f1399fa1221969c7fd-799134117.us-east-1.elb.amazonaws.com",
		Port:         4000,
		Database:     "orderbook_test",
		MaxIdleConns: 10,
		MaxOpenConns: 1500,
	})
	chainClient, _ := chainclient.New(10, "https://rpc.ankr.com/optimism/9c6c678ebcb56da1cb80f7632c7c02264831232c3d53453c7726a611e7ca36d7")

	// 设置测试配置
	testConfig := &config.Config{
		ContractCfg: config.ContractCfg{
			VaultAddress: "0x7d29d1860bD4d3A74bBD9a03C9B043d375311dCb", // 测试vault地址
		},
	}

	orderbookSyncer := New(ctx, testConfig, db, nil, chainClient, 10, "optimism", nil)

	// 测试批量检查NFT授权状态
	nfts := []struct {
		CollectionAddress string
		TokenId           string
	}{
		{"0xe7f1725e7734ce288f8367e1bb143e90bb3f0512", "1"},
		{"0xe7f1725e7734ce288f8367e1bb143e90bb3f0512", "2"},
		{"0xabc123456789abcdef123456789abcdef123456", "3"},
	}

	results, err := orderbookSyncer.CheckMultipleNFTApprovals(nfts)
	if err != nil {
		t.Logf("批量检查授权状态出错: %v", err)
	} else {
		for key, approved := range results {
			t.Logf("NFT %s 授权状态: %v", key, approved)
		}
	}
}
