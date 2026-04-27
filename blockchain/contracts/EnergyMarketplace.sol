// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "@openzeppelin/contracts/access/AccessControl.sol";
import "@openzeppelin/contracts/utils/ReentrancyGuard.sol";
import "./EnergyToken.sol";

/**
 * @title EnergyMarketplace
 * @notice Order-book marketplace for peer-to-peer energy trading.
 *         Producers list energy lots with a price per kWh.
 *         Consumers purchase available lots. After purchase, EnergyTokens are
 *         minted to the producer as a credit for surplus injected into the grid.
 */
contract EnergyMarketplace is AccessControl, ReentrancyGuard {
    bytes32 public constant SETTLER_ROLE = keccak256("SETTLER_ROLE");

    EnergyToken public immutable energyToken;

    struct EnergyLot {
        uint256 id;
        address payable seller;
        uint256 amountKwh;
        uint256 pricePerKwh; // in wei
        bool active;
    }

    uint256 private _lotCounter;
    mapping(uint256 => EnergyLot) public lots;

    event EnergyListed(
        uint256 indexed lotId,
        address indexed seller,
        uint256 amountKwh,
        uint256 pricePerKwh
    );

    event EnergyPurchased(
        uint256 indexed lotId,
        address indexed buyer,
        uint256 amountKwh,
        uint256 totalPrice
    );

    event SettlementCompleted(
        uint256 indexed lotId,
        address indexed buyer,
        address indexed seller,
        uint256 amountKwh
    );

    constructor(address _energyToken) {
        energyToken = EnergyToken(_energyToken);
        _grantRole(DEFAULT_ADMIN_ROLE, msg.sender);
    }

    /**
     * @notice List energy for sale on the marketplace.
     * @param amountKwh Quantity in kWh to sell
     * @param pricePerKwh Price per kWh in wei
     */
    function listEnergy(uint256 amountKwh, uint256 pricePerKwh) external {
        require(amountKwh > 0, "Amount must be > 0");
        require(pricePerKwh > 0, "Price must be > 0");

        _lotCounter++;
        uint256 lotId = _lotCounter;

        lots[lotId] = EnergyLot({
            id: lotId,
            seller: payable(msg.sender),
            amountKwh: amountKwh,
            pricePerKwh: pricePerKwh,
            active: true
        });

        emit EnergyListed(lotId, msg.sender, amountKwh, pricePerKwh);
    }

    /**
     * @notice Purchase an entire energy lot.
     * @param lotId ID of the lot to purchase
     */
    function purchaseEnergy(uint256 lotId) external payable nonReentrant {
        EnergyLot storage lot = lots[lotId];
        require(lot.active, "Lot not active");
        require(msg.sender != lot.seller, "Cannot buy own lot");

        uint256 totalPrice = lot.amountKwh * lot.pricePerKwh;
        require(msg.value >= totalPrice, "Insufficient payment");

        lot.active = false;

        // Refund overpayment
        if (msg.value > totalPrice) {
            (bool refundOk, ) = payable(msg.sender).call{
                value: msg.value - totalPrice
            }("");
            require(refundOk, "Refund failed");
        }

        // Transfer payment to seller
        (bool payOk, ) = lot.seller.call{value: totalPrice}("");
        require(payOk, "Payment to seller failed");

        // Mint energy tokens to producer as credit for surplus
        energyToken.mint(lot.seller, lot.amountKwh);

        emit EnergyPurchased(lotId, msg.sender, lot.amountKwh, totalPrice);
        emit SettlementCompleted(lotId, msg.sender, lot.seller, lot.amountKwh);
    }

    /**
     * @notice Cancel an active listing. Only the seller can cancel.
     * @param lotId ID of the lot to cancel
     */
    function cancelListing(uint256 lotId) external {
        EnergyLot storage lot = lots[lotId];
        require(lot.active, "Lot not active");
        require(msg.sender == lot.seller, "Not the seller");
        lot.active = false;
    }

    /**
     * @notice Get the total number of lots created.
     */
    function lotCount() external view returns (uint256) {
        return _lotCounter;
    }
}