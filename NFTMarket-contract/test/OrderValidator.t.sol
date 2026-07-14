// SPDX-License-Identifier: MIT
pragma solidity ^0.8.35;

import {Test} from "forge-std/Test.sol";
import {OrderValidator} from "../src/OrderValidator.sol";
import {LibOrder, OrderKey} from "../src/libraries/LibOrder.sol";
import {Price} from "../src/libraries/RedBlackTreeLibrary.sol";

/// @dev 测试桩：将内部校验方法暴露为外部可调用接口，便于单测断言。
contract OrderValidatorHarness is OrderValidator {
    function initialize() external initializer {
        __OrderValidator_init("EasySwap", "1");
    }

    function validate(LibOrder.Order memory order, bool isSkipExpiry) external view {
        _validateOrder(order, isSkipExpiry);
    }

    function updateFilledAmount(uint256 newAmount, OrderKey orderKey) external {
        _updateFilledAmount(newAmount, orderKey);
    }

    function cancelOrder(OrderKey orderKey) external {
        _cancelOrder(orderKey);
    }

    function getFilled(OrderKey orderKey) external view returns (uint256) {
        return _getFilledAmount(orderKey);
    }

    function isExpired(uint64 expirationTime) external view returns (bool) {
        return _isExpiredByTimestamp(expirationTime);
    }
}

/// @dev OrderValidator 分合约测试：参数合法性、过期规则、filledAmount 边界。
contract OrderValidatorTest is Test {
    OrderValidatorHarness internal harness;
    address internal maker = makeAddr("maker");
    address internal collection = makeAddr("collection");

    /// @dev 固定时间戳，保证过期相关断言稳定可复现。
    function setUp() external {
        vm.warp(1_000);
        harness = new OrderValidatorHarness();
        harness.initialize();
    }

    /// @dev 构造基础订单，避免每个测试重复拼装结构体。
    function _buildOrder(
        LibOrder.Side side,
        uint128 price,
        address maker_,
        address collection_,
        uint64 expirationTime,
        uint64 salt
    ) internal pure returns (LibOrder.Order memory order) {
        order = LibOrder.Order({
            side: side,
            saleKind: LibOrder.SaleKind.FixedPriceForItem,
            maker: maker_,
            nft: LibOrder.Asset({tokenId: 1, collection: collection_, amount: 1}),
            price: Price.wrap(price),
            expirationTime: expirationTime,
            salt: salt
        });
    }

    /// @dev 边界：maker 为零地址应被拒绝。
    function test_validate_revertsWhenMakerIsZero() external {
        LibOrder.Order memory order = _buildOrder(LibOrder.Side.Bid, 1 ether, address(0), collection, uint64(block.timestamp + 1), 1);

        vm.expectRevert(bytes("OVa: miss maker"));
        harness.validate(order, false);
    }

    /// @dev 边界：过期订单且未启用 skipExpiry 必须回滚。
    function test_validate_revertsWhenExpiredAndNotSkipped() external {
        LibOrder.Order memory order = _buildOrder(LibOrder.Side.Bid, 1 ether, maker, collection, uint64(block.timestamp - 1), 1);

        vm.expectRevert(bytes("OVa: expired"));
        harness.validate(order, false);
    }

    /// @dev 业务语义：启用 skipExpiry 时允许带过期时间的订单继续处理。
    function test_validate_allowsExpiredWhenSkipEnabledBoundary() external view {
        LibOrder.Order memory order = _buildOrder(LibOrder.Side.Bid, 1 ether, maker, collection, uint64(block.timestamp - 1), 1);

        harness.validate(order, true);
    }

    /// @dev 边界：Bid 订单价格不能为 0。
    function test_validate_revertsBidWithZeroPriceBoundary() external {
        LibOrder.Order memory order = _buildOrder(LibOrder.Side.Bid, 0, maker, collection, uint64(block.timestamp + 1), 1);

        vm.expectRevert(bytes("OVa: zero price"));
        harness.validate(order, false);
    }

    /// @dev 边界：List 订单 NFT collection 不能为零地址。
    function test_validate_revertsListWithZeroCollectionBoundary() external {
        LibOrder.Order memory order = _buildOrder(LibOrder.Side.List, 1, maker, address(0), uint64(block.timestamp + 1), 1);

        vm.expectRevert(bytes("OVa: unsupported nft asset"));
        harness.validate(order, false);
    }

    /// @dev 状态边界：filledAmount 正常更新；写入 CANCELLED 值应被拒绝；取消后值为 max。
    function test_updateAndCancelFilledAmount_boundaries() external {
        LibOrder.Order memory order = _buildOrder(LibOrder.Side.Bid, 1, maker, collection, uint64(block.timestamp + 1), 9);
        OrderKey key = LibOrder.hash(order);

        harness.updateFilledAmount(3, key);
        assertEq(harness.getFilled(key), 3);

        vm.expectRevert(bytes("OVa: canceled"));
        harness.updateFilledAmount(type(uint256).max, key);

        harness.cancelOrder(key);
        assertEq(harness.getFilled(key), type(uint256).max);
    }

    /// @dev 过期判断边界：过去时间为过期，0 代表永不过期。
    function test_isExpired_boundaryAtZeroAndPast() external view {
        assertTrue(harness.isExpired(uint64(block.timestamp - 1)));
        assertFalse(harness.isExpired(0));
    }
}
