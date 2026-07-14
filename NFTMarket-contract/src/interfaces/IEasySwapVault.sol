// SPDX-License-Identifier: MIT
pragma solidity ^0.8.35;
import {LibOrder, OrderKey} from "../libraries/LibOrder.sol";
interface IEasySwapVault{
    function balanceOf(OrderKey orderKey) external view returns (uint256 ETHAmount, uint256 tokenId);
    function depositETH(OrderKey orderKey, uint256 ETHAmount) external payable;
    function withdrawETH(OrderKey orderKey, uint256 ETHAmount,address to) external;
    function depositNFT(OrderKey orderKey,address from, address collection,uint256 tokenId) external;
    function withdrawNFT(OrderKey orderKey,address to, address collection,uint256 tokenId) external;
    function editNFT(OrderKey oldOrderKey, OrderKey newOrderKey) external;
    function editETH(OrderKey oldOrderKey, OrderKey newOrderKey, uint256 oldETHAmount, uint256 newETHAmount, address to ) external payable;
    function batchTransferERC721(
        address to,
        LibOrder.NFTInfo[] calldata assets
    ) external;
    function transferERC721(
        address from,
        address to,
        LibOrder.Asset calldata assets
    ) external;
}