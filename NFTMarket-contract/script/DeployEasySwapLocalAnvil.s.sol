// SPDX-License-Identifier: MIT
pragma solidity ^0.8.35;

import {Script, console2} from "forge-std/Script.sol";
import {EasySwapVault} from "../src/EasySwapVault.sol";
import {EasySwapOrderBook} from "../src/EasySwapOrderBook.sol";
import {OrderKey} from "../src/libraries/LibOrder.sol";

/// @notice 本地 Anvil 一键部署 + 验收检查脚本
/// @dev 验收项：owner、protocolShare、Vault<->OrderBook 绑定、初始余额读数
contract DeployEasySwapLocalAnvilScript is Script {
    function run() external {
        uint256 deployerPrivateKey = vm.envUint("DEPLOYER_PRIVATE_KEY");
        uint128 protocolShare = uint128(vm.envOr("PROTOCOL_SHARE", uint256(250)));
        string memory eip712Name = vm.envOr("EIP712_NAME", string("EasySwap"));
        string memory eip712Version = vm.envOr("EIP712_VERSION", string("1"));
        address finalOwner = vm.envOr("FINAL_OWNER", address(0));

        address deployer = vm.addr(deployerPrivateKey);
        address expectedOwner = finalOwner == address(0) ? deployer : finalOwner;

        vm.startBroadcast(deployerPrivateKey);

        EasySwapVault vault = new EasySwapVault();
        vault.initialize();

        EasySwapOrderBook orderBook = new EasySwapOrderBook();
        orderBook.initialize(protocolShare, address(vault), eip712Name, eip712Version);

        vault.setOrderBook(address(orderBook));

        if (finalOwner != address(0)) {
            vault.transferOwnership(finalOwner);
            orderBook.transferOwnership(finalOwner);
        }

        vm.stopBroadcast();

        // 验收检查 1：双合约 owner 应符合预期
        require(vault.owner() == expectedOwner, "CHECK: vault owner mismatch");
        require(orderBook.owner() == expectedOwner, "CHECK: orderBook owner mismatch");

        // 验收检查 2：Vault 的 orderBook 绑定应正确
        require(vault.orderBook() == address(orderBook), "CHECK: vault->orderBook mismatch");

        // 验收检查 3：OrderBook 协议费参数应正确
        require(orderBook.protocolShare() == protocolShare, "CHECK: protocolShare mismatch");

        // 验收检查 4：任意空订单 key 的初始余额应为 0
        (uint256 ethAmount, uint256 tokenId) = vault.balanceOf(OrderKey.wrap(bytes32(0)));
        require(ethAmount == 0 && tokenId == 0, "CHECK: initial vault balance not zero");

        console2.log("[OK] EasySwapVault:", address(vault));
        console2.log("[OK] EasySwapOrderBook:", address(orderBook));
        console2.log("[OK] owner:", expectedOwner);
        console2.log("[OK] protocolShare:", protocolShare);
        console2.log("[OK] local deployment acceptance checks passed");
    }
}
