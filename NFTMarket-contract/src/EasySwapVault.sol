// SPDX-License-Identifier: MIT
pragma solidity ^0.8.35;
/**
 * @title EasySwapVault 资产托管合约，用于NFT交易中的资产托管
 * @dev 业务职责
 *    - 仅允许EasySwapOrderBook合约调用存取款，避免资金任意转出
 *    - 按照订单维度(OrderKey)记录NFT和ETH余额，支持挂单/改单/成交时的资金与NFT流转
 *    - 支持订单编辑：改价和改数量，旧订单与新订单之间转移托管资产
 *    - 支持订单薄直接NFT转账以及用户批量转NFT
 */
import {OwnableUpgradeable} from "@openzeppelin/contracts-upgradeable/access/OwnableUpgradeable.sol";
import {LibTransferSafeUpgradeable, IERC721} from "./libraries/LibTransferSafeUpgradeable.sol";
import {LibOrder, OrderKey} from "./libraries/LibOrder.sol";
import {IEasySwapVault} from "./interfaces/IEasySwapVault.sol";


contract EasySwapVault is IEasySwapVault, OwnableUpgradeable {
    using LibTransferSafeUpgradeable for address;
    using LibTransferSafeUpgradeable for IERC721;
    /// @dev 订单簿合约地址，只有该地址可操作托管资产
    address public orderBook;

    mapping(OrderKey => uint256) public ETHBalance; // 订单维度ETH余额
    mapping(OrderKey => uint256) public NFTBalance; // 订单维度NFT余额，NFTBalance[orderKey][collection] = amount
    /**
     * @dev 仅EasySwap 订单簿可操作托管资产
     */
    modifier onlyEasySwapOrderBook() {
        require(msg.sender == orderBook, "HV: only EasySwap OrderBook");
        _;
    }

    function initialize() external initializer {
        __Ownable_init(_msgSender());
    }
    /// @notice 设置订单簿合约地址（仅管理员）
    function setOrderBook(address newOrderBook) public onlyOwner {
        require(newOrderBook != address(0), "HV: zero address");
        orderBook = newOrderBook;
    }

    /// @notice 查询某订单下托管的 ETH 数量与 NFT tokenId
    function balanceOf(
        OrderKey orderKey
    ) external view returns (uint256 ETHAmount, uint256 tokenId) {
        ETHAmount = ETHBalance[orderKey];
        tokenId = NFTBalance[orderKey];
    }

    /// @notice 订单簿为某订单存入 ETH（如买单挂单时锁定出价）
    function depositETH(
        OrderKey orderKey,
        uint256 ETHAmount
    ) external payable onlyEasySwapOrderBook {
        require(msg.value >= ETHAmount, "HV: not match ETHAmount");
        ETHBalance[orderKey] += msg.value;
    }
    /// @notice 订单簿从某订单提取 ETH 并转给 to（如成交给卖家或撤单退还给买家）
    function withdrawETH(
        OrderKey orderKey,
        uint256 ETHAmount,
        address to
    ) external onlyEasySwapOrderBook {
        ETHBalance[orderKey] -= ETHAmount;
        to.safeTransferETH(ETHAmount);
    }
    /// @notice 订单簿将某 NFT 从 from 转入金库并记入 orderKey（卖单挂单时锁定 NFT）
    function depositNFT(
        OrderKey orderKey,
        address from,
        address collection,
        uint256 tokenId
    ) external onlyEasySwapOrderBook {
        IERC721(collection).safeTransferNFT(from, address(this), tokenId);

        NFTBalance[orderKey] = tokenId;
    }

    /// @notice 订单簿从金库提取某订单的 NFT 并转给 to（如成交给买家或撤单退还给卖家）
    function withdrawNFT(
        OrderKey orderKey,
        address to,
        address collection,
        uint256 tokenId
    ) external onlyEasySwapOrderBook {
        require(NFTBalance[orderKey] == tokenId, "HV: not match tokenId");
        delete NFTBalance[orderKey];

        IERC721(collection).safeTransferNFT(address(this), to, tokenId);
    }
    /// @notice 订单编辑时迁移 ETH：从 oldOrderKey 清空，按新旧金额差多退少补，余额记入 newOrderKey
    function editETH(
        OrderKey oldOrderKey,
        OrderKey newOrderKey,
        uint256 oldETHAmount,
        uint256 newETHAmount,
        address to
    ) external payable onlyEasySwapOrderBook{
        ETHBalance[oldOrderKey] = 0;
        if (oldETHAmount > newETHAmount) {
            ETHBalance[newOrderKey] = newETHAmount;
            to.safeTransferETH(oldETHAmount - newETHAmount);
        } else if (oldETHAmount < newETHAmount) {
            require(
                msg.value >= newETHAmount - oldETHAmount,
                "HV: not match newETHAmount"
            );
            ETHBalance[newOrderKey] = msg.value + oldETHAmount;
        } else {
            ETHBalance[newOrderKey] = oldETHAmount;
        }
    }
    /// @notice 订单编辑时迁移 NFT：将 oldOrderKey 下托管的 NFT 记到 newOrderKey（改价不改 NFT 时使用）
    function editNFT(
        OrderKey oldOrderKey,
        OrderKey newOrderKey
    ) external onlyEasySwapOrderBook {
        NFTBalance[newOrderKey] = NFTBalance[oldOrderKey];
        delete NFTBalance[oldOrderKey];
    }
    /// @notice 订单簿发起的单笔 NFT 转账（如撮合时从卖家转给买家）
    function transferERC721(
        address from,
        address to,
        LibOrder.Asset calldata assets
    ) external onlyEasySwapOrderBook {
        IERC721(assets.collection).safeTransferNFT(from, to, assets.tokenId);
    }
    /// @notice 用户将多笔 NFT 从自己账户批量转给 to（如批量上架时转入金库或指定地址）
    function batchTransferERC721(
        address to,
        LibOrder.NFTInfo[] calldata assets
    ) external {
        for (uint256 i = 0; i < assets.length; ++i) {
            IERC721(assets[i].collection).safeTransferNFT(
                _msgSender(),
                to,
                assets[i].tokenId
            );
        }
    }

    /// @dev 实现 ERC721 接收，使金库能接收 safeTransferFrom 的 NFT
    function onERC721Received(
        address,
        address,
        uint256,
        bytes memory
    ) public virtual returns (bytes4) {
        return this.onERC721Received.selector;
    }
    receive() external payable {}

    uint256[50] private __gap;
}