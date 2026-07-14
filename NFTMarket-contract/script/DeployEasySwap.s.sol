// SPDX-License-Identifier: MIT
pragma solidity ^0.8.35;

import {Script, console2} from "forge-std/Script.sol";
import {EasySwapVault} from "../src/EasySwapVault.sol";
import {EasySwapOrderBook} from "../src/EasySwapOrderBook.sol";

/// @notice EasySwap 部署脚本（Foundry）
/// @dev 部署顺序：Vault -> initialize -> OrderBook -> initialize -> setOrderBook
/// @dev 该脚本假设当前合约使用的是可初始化模式（initializer），不是代理部署。
contract DeployEasySwapScript is Script {
    function run() external {
        // 必填：部署者私钥
        uint256 deployerPrivateKey = vm.envUint("DEPLOYER_PRIVATE_KEY");

        // 可选：协议费比例，默认 250（2.5%）
        uint128 protocolShare = uint128(vm.envOr("PROTOCOL_SHARE", uint256(250)));

        // 可选：EIP712 域信息，默认 EasySwap / 1
        string memory eip712Name = vm.envOr("EIP712_NAME", string("EasySwap"));
        string memory eip712Version = vm.envOr("EIP712_VERSION", string("1"));

        // 可选：部署后转移 owner（不填则保持部署者为 owner）
        address finalOwner = vm.envOr("FINAL_OWNER", address(0));

        vm.startBroadcast(deployerPrivateKey);

        // 1) 部署 Vault 并初始化 owner
        EasySwapVault vault = new EasySwapVault();
        vault.initialize();

        // 2) 部署 OrderBook 并初始化（会在内部绑定 vault）
        EasySwapOrderBook orderBook = new EasySwapOrderBook();
        orderBook.initialize(protocolShare, address(vault), eip712Name, eip712Version);

        // 3) 回写 Vault 的 orderBook 地址，建立双向关联
        vault.setOrderBook(address(orderBook));

        // 4) 可选：统一转移 owner 到运营地址
        if (finalOwner != address(0)) {
            vault.transferOwnership(finalOwner);
            orderBook.transferOwnership(finalOwner);
        }

        vm.stopBroadcast();

        console2.log("EasySwapVault deployed at:", address(vault));
        console2.log("EasySwapOrderBook deployed at:", address(orderBook));
        console2.log("Protocol share:", protocolShare);
        console2.log("EIP712 name:", eip712Name);
        console2.log("EIP712 version:", eip712Version);
        if (finalOwner != address(0)) {
            console2.log("Ownership transferred to:", finalOwner);
        }
    }
}
