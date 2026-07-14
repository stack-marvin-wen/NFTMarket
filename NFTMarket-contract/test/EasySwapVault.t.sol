// SPDX-License-Identifier: MIT
pragma solidity ^0.8.35;

import {Test} from "forge-std/Test.sol";
import {EasySwapVault} from "../src/EasySwapVault.sol";
import {LibOrder, OrderKey} from "../src/libraries/LibOrder.sol";
import {ERC721} from "@openzeppelin/contracts/token/ERC721/ERC721.sol";

/// @dev 仅用于 Vault 测试的最小 ERC721 实现。
contract MockERC721 is ERC721 {
    constructor() ERC721("Mock", "MCK") {}

    function mint(address to, uint256 tokenId) external {
        _mint(to, tokenId);
    }
}

/// @dev EasySwapVault 分合约测试：权限控制、ETH/NFT 托管流转、改价资产迁移边界。
contract EasySwapVaultTest is Test {
    EasySwapVault internal vault;
    MockERC721 internal nft;

    address internal owner = makeAddr("owner");
    address internal orderBook = makeAddr("orderBook");
    address internal user = makeAddr("user");
    address internal recipient = makeAddr("recipient");

    /// @dev 初始化资金与角色，部署 Vault 并设置 orderBook，再部署测试 NFT。
    function setUp() external {
        vm.deal(user, 10 ether);
        vm.deal(orderBook, 10 ether);

        vm.prank(owner);
        vault = new EasySwapVault();

        vm.prank(owner);
        vault.initialize();

        vm.prank(owner);
        vault.setOrderBook(orderBook);

        nft = new MockERC721();
    }

    /// @dev 边界：仅 owner 可设置 orderBook；newOrderBook 不能为零地址。
    function test_setOrderBook_boundaries() external {
        vm.prank(owner);
        vault.setOrderBook(orderBook);
        assertEq(vault.orderBook(), orderBook);

        vm.prank(user);
        vm.expectRevert();
        vault.setOrderBook(makeAddr("x"));

        vm.prank(owner);
        vm.expectRevert(bytes("HV: zero address"));
        vault.setOrderBook(address(0));
    }

    /// @dev ETH 托管路径：onlyOrderBook 限制、入金金额校验、出金后余额与收款变化。
    function test_depositAndWithdrawETH_onlyOrderBookAndAmountChecks() external {
        OrderKey key = OrderKey.wrap(keccak256("k1"));

        vm.prank(user);
        vm.expectRevert(bytes("HV: only EasySwap OrderBook"));
        vault.depositETH{value: 1 ether}(key, 1 ether);

        vm.prank(orderBook);
        vm.expectRevert(bytes("HV: not match ETHAmount"));
        vault.depositETH{value: 1 ether}(key, 2 ether);

        vm.prank(orderBook);
        vault.depositETH{value: 2 ether}(key, 2 ether);
        (uint256 ethAmount,) = vault.balanceOf(key);
        assertEq(ethAmount, 2 ether);

        uint256 beforeBal = recipient.balance;
        vm.prank(orderBook);
        vault.withdrawETH(key, 1 ether, recipient);

        (uint256 afterAmount,) = vault.balanceOf(key);
        assertEq(afterAmount, 1 ether);
        assertEq(recipient.balance - beforeBal, 1 ether);
    }

    /// @dev NFT 托管路径：存入成功、错误 tokenId 提现回滚、正确 tokenId 提现并清余额。
    function test_depositAndWithdrawNFT_boundaries() external {
        OrderKey key = OrderKey.wrap(keccak256("k2"));

        nft.mint(user, 1);
        vm.prank(user);
        nft.approve(address(vault), 1);

        vm.prank(orderBook);
        vault.depositNFT(key, user, address(nft), 1);

        (, uint256 tokenId) = vault.balanceOf(key);
        assertEq(tokenId, 1);
        assertEq(nft.ownerOf(1), address(vault));

        vm.prank(orderBook);
        vm.expectRevert(bytes("HV: not match tokenId"));
        vault.withdrawNFT(key, recipient, address(nft), 2);

        vm.prank(orderBook);
        vault.withdrawNFT(key, recipient, address(nft), 1);

        (, uint256 clearedTokenId) = vault.balanceOf(key);
        assertEq(clearedTokenId, 0);
        assertEq(nft.ownerOf(1), recipient);
    }

    /// @dev editETH 边界：降价退差、升价补差、补款不足时回滚。
    function test_editETH_boundaries_refundAndTopup() external {
        OrderKey oldKey = OrderKey.wrap(keccak256("old"));
        OrderKey newKey = OrderKey.wrap(keccak256("new"));

        vm.prank(orderBook);
        vault.depositETH{value: 2 ether}(oldKey, 2 ether);

        uint256 beforeRecipient = recipient.balance;

        vm.prank(orderBook);
        vault.editETH(oldKey, newKey, 2 ether, 1 ether, recipient);

        (uint256 oldBal,) = vault.balanceOf(oldKey);
        (uint256 newBal,) = vault.balanceOf(newKey);
        assertEq(oldBal, 0);
        assertEq(newBal, 1 ether);
        assertEq(recipient.balance - beforeRecipient, 1 ether);

        OrderKey newKey2 = OrderKey.wrap(keccak256("new2"));
        vm.prank(orderBook);
        vm.expectRevert(bytes("HV: not match newETHAmount"));
        vault.editETH{value: 0.4 ether}(newKey, newKey2, 1 ether, 1.5 ether, recipient);

        vm.prank(orderBook);
        vault.editETH{value: 0.5 ether}(newKey, newKey2, 1 ether, 1.5 ether, recipient);

        (uint256 newBal2,) = vault.balanceOf(newKey2);
        assertEq(newBal2, 1.5 ether);
    }

    /// @dev 用户批量 NFT 转移路径：两笔资产均应转到目标地址。
    function test_batchTransferERC721_userCanBatchTransfer() external {
        nft.mint(user, 10);
        nft.mint(user, 11);

        vm.startPrank(user);
        nft.approve(address(vault), 10);
        nft.approve(address(vault), 11);

        LibOrder.NFTInfo[] memory assets = new LibOrder.NFTInfo[](2);
        assets[0] = LibOrder.NFTInfo({tokenId: 10, collection: address(nft)});
        assets[1] = LibOrder.NFTInfo({tokenId: 11, collection: address(nft)});

        vault.batchTransferERC721(recipient, assets);
        vm.stopPrank();

        assertEq(nft.ownerOf(10), recipient);
        assertEq(nft.ownerOf(11), recipient);
    }
}
