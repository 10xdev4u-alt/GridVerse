const { expect } = require("chai");
const { ethers } = require("hardhat");

describe("Energy Trading", function () {
  let EnergyToken, energyToken;
  let EnergyMarketplace, marketplace;
  let Settlement, settlement;
  let owner, producer, consumer, attacker;

  const ONE_ETHER = ethers.parseEther("1.0");
  const LISTING_AMOUNT = 100n;    // kWh
  const PRICE_PER_KWH = ethers.parseEther("0.01"); // 0.01 ETH per kWh
  const TOTAL_PRICE = LISTING_AMOUNT * PRICE_PER_KWH / BigInt(1);

  beforeEach(async function () {
    [owner, producer, consumer, attacker] = await ethers.getSigners();

    // Deploy EnergyToken with marketplace as minter
    EnergyToken = await ethers.getContractFactory("EnergyToken");
    energyToken = await EnergyToken.deploy(owner.address);
    await energyToken.waitForDeployment();

    // Deploy marketplace linked to token
    EnergyMarketplace = await ethers.getContractFactory("EnergyMarketplace");
    marketplace = await EnergyMarketplace.deploy(energyToken.target);
    await marketplace.waitForDeployment();

    // Grant MINTER_ROLE to marketplace
    const MINTER_ROLE = await energyToken.MINTER_ROLE();
    await energyToken.grantRole(MINTER_ROLE, marketplace.target);

    // Deploy settlement linked to token
    Settlement = await ethers.getContractFactory("Settlement");
    settlement = await Settlement.deploy(energyToken.target);
    await settlement.waitForDeployment();
  });

  describe("EnergyToken", function () {
    it("should have correct name and symbol", async function () {
      expect(await energyToken.name()).to.equal("EnergyToken");
      expect(await energyToken.symbol()).to.equal("ENRG");
    });

    it("should allow minter to mint tokens", async function () {
      await marketplace.connect(producer).listEnergy(LISTING_AMOUNT, PRICE_PER_KWH);
      await marketplace.connect(consumer).purchaseEnergy(1, { value: ONE_ETHER });

      const balance = await energyToken.balanceOf(producer.address);
      expect(balance).to.equal(LISTING_AMOUNT);
    });

    it("should revert when non-minter tries to mint", async function () {
      const MINTER_ROLE = await energyToken.MINTER_ROLE();
      await expect(
        energyToken.connect(attacker).mint(attacker.address, 100)
      ).to.be.reverted;
    });

    it("should allow token holder to burn their own tokens", async function () {
      // First get some tokens via a marketplace trade
      await marketplace.connect(producer).listEnergy(LISTING_AMOUNT, PRICE_PER_KWH);
      await marketplace.connect(consumer).purchaseEnergy(1, { value: ONE_ETHER });

      const initialBalance = await energyToken.balanceOf(producer.address);
      const burnAmount = 50n;

      await energyToken.connect(producer).burn(burnAmount);

      const finalBalance = await energyToken.balanceOf(producer.address);
      expect(finalBalance).to.equal(initialBalance - burnAmount);
    });
  });

  describe("EnergyMarketplace", function () {
    it("should emit EnergyListed event when listing", async function () {
      await expect(
        marketplace.connect(producer).listEnergy(LISTING_AMOUNT, PRICE_PER_KWH)
      )
        .to.emit(marketplace, "EnergyListed")
        .withArgs(1, producer.address, LISTING_AMOUNT, PRICE_PER_KWH);
    });

    it("should emit EnergyPurchased and SettlementCompleted on purchase", async function () {
      await marketplace.connect(producer).listEnergy(LISTING_AMOUNT, PRICE_PER_KWH);

      const expectedTotal = LISTING_AMOUNT * PRICE_PER_KWH / BigInt(1);

      await expect(
        marketplace.connect(consumer).purchaseEnergy(1, { value: ONE_ETHER })
      )
        .to.emit(marketplace, "EnergyPurchased")
        .withArgs(1, consumer.address, LISTING_AMOUNT, expectedTotal)
        .and.to.emit(marketplace, "SettlementCompleted")
        .withArgs(1, consumer.address, producer.address, LISTING_AMOUNT);
    });

    it("should transfer ETH to seller on purchase", async function () {
      await marketplace.connect(producer).listEnergy(LISTING_AMOUNT, PRICE_PER_KWH);

      const sellerBalanceBefore = await ethers.provider.getBalance(producer.address);

      await marketplace.connect(consumer).purchaseEnergy(1, { value: ONE_ETHER });

      const sellerBalanceAfter = await ethers.provider.getBalance(producer.address);
      const expectedTotal = LISTING_AMOUNT * PRICE_PER_KWH / BigInt(1);
      expect(sellerBalanceAfter - sellerBalanceBefore).to.equal(expectedTotal);
    });

    it("should refund overpayment", async function () {
      await marketplace.connect(producer).listEnergy(LISTING_AMOUNT, PRICE_PER_KWH);

      const overpayValue = ethers.parseEther("2.0"); // double the needed amount
      const balanceBefore = await ethers.provider.getBalance(consumer.address);

      const tx = await marketplace.connect(consumer).purchaseEnergy(1, {
        value: overpayValue,
      });
      const receipt = await tx.wait();

      const gasCost = receipt.gasUsed * receipt.gasPrice;
      const balanceAfter = await ethers.provider.getBalance(consumer.address);

      // Consumer paid totalPrice + gas
      const expectedTotal = LISTING_AMOUNT * PRICE_PER_KWH / BigInt(1);
      const spent = balanceBefore - balanceAfter;
      const valueSent = overpayValue - (balanceBefore - balanceAfter - gasCost);
      // Rough check — the refund was processed
      expect(spent).to.be.closeTo(expectedTotal + gasCost, ethers.parseEther("0.01"));
    });

    it("should revert on purchase of inactive lot", async function () {
      await marketplace.connect(producer).listEnergy(LISTING_AMOUNT, PRICE_PER_KWH);
      await marketplace.connect(consumer).purchaseEnergy(1, { value: ONE_ETHER });

      await expect(
        marketplace.connect(consumer).purchaseEnergy(1, { value: ONE_ETHER })
      ).to.be.revertedWith("Lot not active");
    });

    it("should revert when trying to buy own lot", async function () {
      await marketplace.connect(producer).listEnergy(LISTING_AMOUNT, PRICE_PER_KWH);
      await expect(
        marketplace.connect(producer).purchaseEnergy(1, { value: ONE_ETHER })
      ).to.be.revertedWith("Cannot buy own lot");
    });

    it("should revert on insufficient payment", async function () {
      await marketplace.connect(producer).listEnergy(LISTING_AMOUNT, PRICE_PER_KWH);
      await expect(
        marketplace.connect(consumer).purchaseEnergy(1, { value: 1 }) // 1 wei
      ).to.be.revertedWith("Insufficient payment");
    });

    it("should allow seller to cancel listing", async function () {
      await marketplace.connect(producer).listEnergy(LISTING_AMOUNT, PRICE_PER_KWH);
      await marketplace.connect(producer).cancelListing(1);

      const lot = await marketplace.lots(1);
      expect(lot.active).to.be.false;
    });

    it("should revert when non-seller tries to cancel", async function () {
      await marketplace.connect(producer).listEnergy(LISTING_AMOUNT, PRICE_PER_KWH);
      await expect(
        marketplace.connect(consumer).cancelListing(1)
      ).to.be.revertedWith("Not the seller");
    });

    it("should prevent reentrancy via nonReentrant on purchase", async function () {
      // Producer lists lot 1 (the target the attacker will try to re-enter and buy)
      await marketplace.connect(producer).listEnergy(LISTING_AMOUNT, PRICE_PER_KWH);

      // Deploy the attacker contract
      const ReentrancyAttacker = await ethers.getContractFactory("ReentrancyAttacker");
      const attackerContract = await ReentrancyAttacker.deploy(marketplace.target);
      await attackerContract.waitForDeployment();

      // Attacker lists its own lot (lot 2) and remembers lot 1 for reentrancy target
      await attackerContract.setup(1, { value: PRICE_PER_KWH });

      // Consumer buys the attacker's lot (lot 2).
      // ETH sent to attacker, receive() uses low-level call to attempt reentrancy
      // on lot 1. nonReentrant should block the reentrant purchaseEnergy call.
      const totalPrice = 1n * PRICE_PER_KWH / BigInt(1);
      await expect(
        marketplace.connect(consumer).purchaseEnergy(2, { value: totalPrice })
      ).to.emit(marketplace, "EnergyPurchased");

      // Lot 2 should be completed
      const lot2 = await marketplace.lots(2);
      expect(lot2.active).to.be.false;

      // Lot 1 should still be active -- reentrancy was blocked
      const lot1 = await marketplace.lots(1);
      expect(lot1.active).to.be.true;

      // Attacker's reentrancy was blocked by nonReentrant
      expect(await attackerContract.reentrancyBlocked()).to.be.true;
    });

    it("should update lot counter correctly", async function () {
      expect(await marketplace.lotCount()).to.equal(0);
      await marketplace.connect(producer).listEnergy(50, PRICE_PER_KWH);
      await marketplace.connect(producer).listEnergy(30, PRICE_PER_KWH);
      expect(await marketplace.lotCount()).to.equal(2);
    });
  });

  describe("Settlement", function () {
    it("should emit TradeMatched event", async function () {
      await expect(
        settlement.matchTrade(consumer.address, producer.address, 50, PRICE_PER_KWH)
      )
        .to.emit(settlement, "TradeMatched")
        .withArgs(1, consumer.address, producer.address, 50, 50n * PRICE_PER_KWH / BigInt(1));
    });

    it("should execute settlement and emit event", async function () {
      // Mint tokens to consumer and approve settlement contract
      const MINTER_ROLE = await energyToken.MINTER_ROLE();
      await energyToken.grantRole(MINTER_ROLE, owner.address);
      await energyToken.mint(consumer.address, 200);
      await energyToken.connect(consumer).approve(settlement.target, 200);

      await settlement.matchTrade(consumer.address, producer.address, 50, PRICE_PER_KWH);

      await expect(settlement.executeSettlement(1))
        .to.emit(settlement, "SettlementExecuted")
        .withArgs(1, consumer.address, producer.address, 50);
    });

    it("should transfer tokens after settlement", async function () {
      const MINTER_ROLE = await energyToken.MINTER_ROLE();
      await energyToken.grantRole(MINTER_ROLE, owner.address);
      await energyToken.mint(consumer.address, 200);
      await energyToken.connect(consumer).approve(settlement.target, 200);

      await settlement.matchTrade(consumer.address, producer.address, 50, PRICE_PER_KWH);

      const sellerBalanceBefore = await energyToken.balanceOf(producer.address);
      await settlement.executeSettlement(1);
      const sellerBalanceAfter = await energyToken.balanceOf(producer.address);

      expect(sellerBalanceAfter - sellerBalanceBefore).to.equal(50);
    });

    it("should revert on double settlement", async function () {
      const MINTER_ROLE = await energyToken.MINTER_ROLE();
      await energyToken.grantRole(MINTER_ROLE, owner.address);
      await energyToken.mint(consumer.address, 200);
      await energyToken.connect(consumer).approve(settlement.target, 200);

      await settlement.matchTrade(consumer.address, producer.address, 50, PRICE_PER_KWH);
      await settlement.executeSettlement(1);

      await expect(settlement.executeSettlement(1)).to.be.revertedWith("Already settled");
    });

    it("should revert when paused", async function () {
      await settlement.pause();
      await expect(
        settlement.matchTrade(consumer.address, producer.address, 50, PRICE_PER_KWH)
      ).to.be.reverted;
    });

    it("should unpause and work normally", async function () {
      await settlement.pause();
      await settlement.unpause();

      await expect(
        settlement.matchTrade(consumer.address, producer.address, 50, PRICE_PER_KWH)
      ).to.emit(settlement, "TradeMatched");
    });

    it("should revert when non-operator tries to match trade", async function () {
      await expect(
        settlement.connect(attacker).matchTrade(
          consumer.address,
          producer.address,
          50,
          PRICE_PER_KWH
        )
      ).to.be.reverted;
    });

    it("should reject zero addresses in matchTrade", async function () {
      await expect(
        settlement.matchTrade(ethers.ZeroAddress, producer.address, 50, PRICE_PER_KWH)
      ).to.be.revertedWith("Invalid buyer");
    });
  });
});