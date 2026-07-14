// SPDX-License-Identifier: MIT
pragma solidity ^0.8.35;

import {Test} from "forge-std/Test.sol";
import {ProtocolManager} from "../src/ProtocolManager.sol";
import {LibPayInfo} from "../src/libraries/LibPayInfo.sol";

/// @dev 暴露初始化入口，便于在测试中直接驱动 ProtocolManager 的内部初始化流程。
contract ProtocolManagerHarness is ProtocolManager {
    function initialize(address owner_, uint128 newProtocolShare) external initializer {
        __Ownable_init(owner_);
        __ProtocolManager_init(newProtocolShare);
    }
}

/// @dev ProtocolManager 分合约测试：聚焦 owner 权限与手续费比例边界。
contract ProtocolManagerTest is Test {
    ProtocolManagerHarness internal harness;
    // owner: 合约管理员，拥有设置协议费的权限
    address internal owner = makeAddr("owner");
    // alice: 普通用户，用来验证无权限场景
    address internal alice = makeAddr("alice");

    /// @dev 默认初始化为 owner=owner, protocolShare=250。
    function setUp() external {
        // 1) 部署测试桩
        harness = new ProtocolManagerHarness();

        // 2) 初始化：指定 owner 和初始协议费比例
        harness.initialize(owner, 250);
    }

    /// @dev 正常路径：初始化值应正确落盘。
    function test_initialize_setsProtocolShare() external view {
        // Then: 初始化后 protocolShare 应该等于 250
        assertEq(harness.protocolShare(), 250);
    }

    /// @dev 权限边界：非 owner 调整协议费应回滚。
    function test_setProtocolShare_revertsForNonOwner() external {
        // Given: 由普通用户 alice 发起调用
        vm.prank(alice);

        // Expect: 这次调用应回滚（权限不足）
        vm.expectRevert();

        // When: 尝试设置新的协议费
        harness.setProtocolShare(300);
    }

    /// @dev 上界边界：超过 MAX_PROTOCOL_SHARE 必须回滚。
    function test_setProtocolShare_revertsWhenExceedingMaxBoundary() external {
        // Given: 由 owner 发起，排除权限因素
        vm.prank(owner);

        // Given: 构造一个“刚好超过上限”的值
        uint128 overMax = LibPayInfo.MAX_PROTOCOL_SHARE + 1;

        // Expect: 超过上限应该报指定错误
        vm.expectRevert(bytes("PM: exceed max protocol share"));

        // When: 设置超过上限的协议费
        harness.setProtocolShare(overMax);
    }

    /// @dev 上界边界：等于 MAX_PROTOCOL_SHARE 允许设置。
    function test_setProtocolShare_allowsExactMaxBoundary() external {
        // Given: 由 owner 发起
        vm.prank(owner);

        // Given: 使用“等于上限”的边界值
        uint128 maxBoundary = LibPayInfo.MAX_PROTOCOL_SHARE;

        // When: 设置为边界值
        harness.setProtocolShare(maxBoundary);

        // Then: 应设置成功且值准确
        assertEq(harness.protocolShare(), maxBoundary);
    }
}
