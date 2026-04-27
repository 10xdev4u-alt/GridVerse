// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "@openzeppelin/contracts/access/AccessControl.sol";
import "@openzeppelin/contracts/utils/Pausable.sol";
import "@openzeppelin/contracts/utils/ReentrancyGuard.sol";
import "./EnergyToken.sol";

/**
 * @title Settlement
 * @notice Auto-settlement contract that transfers tokens from buyer to seller
 *         after a trade match. Includes reentrancy protection and pausability.
 */
contract Settlement is AccessControl, Pausable, ReentrancyGuard {
    bytes32 public constant OPERATOR_ROLE = keccak256("OPERATOR_ROLE");

    EnergyToken public immutable energyToken;

    struct SettlementRecord {
        uint256 id;
        address buyer;
        address payable seller;
        uint256 amountKwh;
        uint256 pricePerKwh;
        uint256 totalPrice;
        uint256 timestamp;
        bool settled;
    }

    uint256 private _settlementCounter;
    mapping(uint256 => SettlementRecord) public settlements;

    event TradeMatched(
        uint256 indexed settlementId,
        address indexed buyer,
        address indexed seller,
        uint256 amountKwh,
        uint256 totalPrice
    );

    event SettlementExecuted(
        uint256 indexed settlementId,
        address indexed buyer,
        address indexed seller,
        uint256 amountKwh
    );

    constructor(address _energyToken) {
        energyToken = EnergyToken(_energyToken);
        _grantRole(DEFAULT_ADMIN_ROLE, msg.sender);
        _grantRole(OPERATOR_ROLE, msg.sender);
    }

    /**
     * @notice Match a trade and create a settlement record.
     * @param buyer Address of the buyer
     * @param seller Address of the seller
     * @param amountKwh Energy amount in kWh
     * @param pricePerKwh Price per kWh in wei
     */
    function matchTrade(
        address buyer,
        address payable seller,
        uint256 amountKwh,
        uint256 pricePerKwh
    ) external whenNotPaused onlyRole(OPERATOR_ROLE) returns (uint256) {
        require(buyer != address(0), "Invalid buyer");
        require(seller != address(0), "Invalid seller");
        require(amountKwh > 0, "Amount must be > 0");
        require(pricePerKwh > 0, "Price must be > 0");
        require(buyer != seller, "Buyer cannot be seller");

        _settlementCounter++;
        uint256 settlementId = _settlementCounter;
        uint256 totalPrice = amountKwh * pricePerKwh;

        settlements[settlementId] = SettlementRecord({
            id: settlementId,
            buyer: buyer,
            seller: seller,
            amountKwh: amountKwh,
            pricePerKwh: pricePerKwh,
            totalPrice: totalPrice,
            timestamp: block.timestamp,
            settled: false
        });

        emit TradeMatched(settlementId, buyer, seller, amountKwh, totalPrice);

        return settlementId;
    }

    /**
     * @notice Execute settlement: transfer tokens from buyer to seller.
     *         Protected against reentrancy.
     * @param settlementId ID of the settlement record
     */
    function executeSettlement(
        uint256 settlementId
    ) external whenNotPaused nonReentrant {
        SettlementRecord storage record = settlements[settlementId];
        require(record.id != 0, "Settlement does not exist");
        require(!record.settled, "Already settled");

        // Transfer tokens: buyer must have approved this contract
        bool success = energyToken.transferFrom(
            record.buyer,
            record.seller,
            record.amountKwh
        );
        require(success, "Token transfer failed");

        record.settled = true;

        emit SettlementExecuted(
            settlementId,
            record.buyer,
            record.seller,
            record.amountKwh
        );
    }

    /**
     * @notice Pause all settlement operations. Only admin.
     */
    function pause() external onlyRole(DEFAULT_ADMIN_ROLE) {
        _pause();
    }

    /**
     * @notice Unpause all settlement operations. Only admin.
     */
    function unpause() external onlyRole(DEFAULT_ADMIN_ROLE) {
        _unpause();
    }

    /**
     * @notice Get the total number of settlement records.
     */
    function settlementCount() external view returns (uint256) {
        return _settlementCounter;
    }
}