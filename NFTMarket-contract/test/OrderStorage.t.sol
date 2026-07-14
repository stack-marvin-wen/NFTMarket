// SPDX-License-Identifier: MIT
pragma solidity ^0.8.35;

import {Test} from "forge-std/Test.sol";
import {OrderStorage} from "../src/OrderStorage.sol";
import {LibOrder, OrderKey} from "../src/libraries/LibOrder.sol";
import {Price} from "../src/libraries/RedBlackTreeLibrary.sol";

/// @dev 测试桩：暴露 _addOrder/_removeOrder 供单测直接调用。
contract OrderStorageHarness is OrderStorage {
    function initialize() external initializer {
        __OrderStorage_init();
    }

    function addOrder(LibOrder.Order memory order) external returns (OrderKey) {
        return _addOrder(order);
    }

    function removeOrder(LibOrder.Order memory order) external returns (OrderKey) {
        return _removeOrder(order);
    }

    function getStoredOrder(OrderKey key) external view returns (LibOrder.Order memory order, OrderKey next) {
        LibOrder.DBOrder storage dbOrder = orders[key];
        order = dbOrder.order;
        next = dbOrder.next;
    }
}

/// @dev OrderStorage 分合约测试：价格优先、重复插入、删除边界、过期跳过。
contract OrderStorageTest is Test {
    OrderStorageHarness internal harness;
    address internal collection = makeAddr("collection");
    address internal maker = makeAddr("maker");

    /// @dev 固定时间戳，确保“过期订单”测试稳定。
    function setUp() external {
        vm.warp(1_000);
        harness = new OrderStorageHarness();
        harness.initialize();
    }

    /// @dev 构造订单工具函数：默认同 collection 与 maker，只变更关键维度。
    function _order(
        uint64 salt,
        uint128 price,
        LibOrder.Side side,
        uint64 expirationTime
    ) internal view returns (LibOrder.Order memory order) {
        order = LibOrder.Order({
            side: side,
            saleKind: LibOrder.SaleKind.FixedPriceForItem,
            maker: maker,
            nft: LibOrder.Asset({tokenId: 7, collection: collection, amount: 1}),
            price: Price.wrap(price),
            expirationTime: expirationTime,
            salt: salt
        });
    }

    /// @dev 正常路径：Bid 取最高价，List 取最低价。
    function test_addOrder_setsBestPrice_forBidAndList() external {
        harness.addOrder(_order(1, 100, LibOrder.Side.Bid, uint64(block.timestamp + 1 hours)));
        harness.addOrder(_order(2, 150, LibOrder.Side.Bid, uint64(block.timestamp + 1 hours)));
        harness.addOrder(_order(3, 50, LibOrder.Side.List, uint64(block.timestamp + 1 hours)));
        harness.addOrder(_order(4, 20, LibOrder.Side.List, uint64(block.timestamp + 1 hours)));

        assertEq(Price.unwrap(harness.getBestPrice(collection, LibOrder.Side.Bid)), 150);
        assertEq(Price.unwrap(harness.getBestPrice(collection, LibOrder.Side.List)), 20);
    }

    /// @dev 边界：同一订单重复插入应回滚。
    function test_addOrder_revertsOnDuplicateBoundary() external {
        LibOrder.Order memory order = _order(11, 88, LibOrder.Side.Bid, uint64(block.timestamp + 1 hours));

        harness.addOrder(order);

        vm.expectRevert();
        harness.addOrder(order);
    }

    /// @dev 边界：删除不存在订单必须回滚。
    function test_removeOrder_revertsWhenMissingBoundary() external {
        LibOrder.Order memory order = _order(12, 90, LibOrder.Side.Bid, uint64(block.timestamp + 1 hours));

        vm.expectRevert(bytes("Cannot remove missing order"));
        harness.removeOrder(order);
    }

    /// @dev 边界：删除价格档最后一单后，该价格档应被清空。
    function test_removeOrder_clearsPriceLevelWhenLastOrderRemoved() external {
        LibOrder.Order memory order = _order(13, 77, LibOrder.Side.Bid, uint64(block.timestamp + 1 hours));
        harness.addOrder(order);

        harness.removeOrder(order);

        assertEq(Price.unwrap(harness.getBestPrice(collection, LibOrder.Side.Bid)), 0);
    }

    /// @dev 过期边界：best order 查询应跳过过期订单。
    function test_getBestOrder_skipsExpiredBoundary() external {
        harness.addOrder(_order(14, 100, LibOrder.Side.Bid, uint64(block.timestamp - 1)));
        harness.addOrder(_order(15, 99, LibOrder.Side.Bid, uint64(block.timestamp + 1 hours)));

        LibOrder.Order memory best = harness.getBestOrder(collection, 7, LibOrder.Side.Bid, LibOrder.SaleKind.FixedPriceForItem);

        assertEq(best.salt, 15);
    }
}
