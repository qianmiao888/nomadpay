// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/// @title NomadPay - direct invoice payments with an indexable receipt
contract NomadPay {
    event InvoicePaid(bytes32 indexed invoiceId, address indexed payer, address indexed recipient, uint256 amount);

    function pay(bytes32 invoiceId, address payable recipient) external payable {
        require(invoiceId != bytes32(0), "invalid invoice");
        require(recipient != address(0), "invalid recipient");
        require(msg.value > 0, "zero payment");
        (bool sent, ) = recipient.call{value: msg.value}("");
        require(sent, "payment failed");
        emit InvoicePaid(invoiceId, msg.sender, recipient, msg.value);
    }
}
